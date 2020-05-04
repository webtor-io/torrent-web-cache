package services

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type Web struct {
	host string
	port int
	ln   net.Listener
	rp   *ReaderPool
}

const (
	WEB_HOST_FLAG = "host"
	WEB_PORT_FLAG = "port"
)

func NewWeb(c *cli.Context, rp *ReaderPool) *Web {
	return &Web{host: c.String(WEB_HOST_FLAG), port: c.Int(WEB_PORT_FLAG), rp: rp}
}

func RegisterWebFlags(c *cli.App) {
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:  WEB_HOST_FLAG,
		Usage: "listening host",
		Value: "",
	})
	c.Flags = append(c.Flags, cli.IntFlag{
		Name:  WEB_PORT_FLAG,
		Usage: "http listening port",
		Value: 8080,
	})
}

func getSourceURL(r *http.Request) string {
	// return "https://api.webtor.io/08ada5a7a6183aae1e09d831df6748d566095a10/Sintel%2FSintel.mp4?download-id=47f73e5c3f7a03130861166edc5ff2ad&user-id=47ab189bb92e6cb478f39c5ece3789e9&token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhZ2VudCI6Ik1vemlsbGEvNS4wIChNYWNpbnRvc2g7IEludGVsIE1hYyBPUyBYIDEwXzE1XzMpIEFwcGxlV2ViS2l0LzUzNy4zNiAoS0hUTUwsIGxpa2UgR2Vja28pIENocm9tZS84MS4wLjQwNDQuMTI5IFNhZmFyaS81MzcuMzYiLCJleHAiOjE1ODg2NDM5OTcsInJhdGUiOiI1ME0iLCJncmFjZSI6MzYwMCwicHJlc2V0IjoidWx0cmFmYXN0In0.OLNri5oI6JJwJmseZd_8WoOwIxKpRu2jEGkyOD76Qc4&api-key=8acbcf1e-732c-4574-a3bf-27e6a85b86f1"
	return r.Header.Get("X-Source-Url")
}

func (s *Web) Serve() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "Failed to web listen to tcp connection")
	}
	s.ln = ln
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url := getSourceURL(r)
		if url == "" {
			log.Error("No source url provided")
			w.WriteHeader(500)
			return
		}
		re, err := s.rp.Get(url)
		if err != nil {
			log.WithError(err).Errorf("Failed get reader for url=%v", url)
			w.WriteHeader(500)
			return
		}
		rr, err := re.Ready()
		if err != nil {
			log.WithError(err).Errorf("Failed get reader ready state for url=%v", url)
			w.WriteHeader(500)
			return
		}
		if !rr {
			http.Redirect(w, r, re.RedirectURL(), 302)
			return
		}
		var rs io.ReadSeeker
		if r.Header.Get("X-Download-Rate") != "" {
			rate, err := bytefmt.ToBytes(r.Header.Get("X-Download-Rate"))
			if err != nil {
				log.WithError(err).Error("Wrong download rate")
				http.Error(w, "Wrong download rate", http.StatusInternalServerError)
				return
			}
			rs = NewThrottledReader(re, rate)
		} else {
			rs = re
		}
		http.ServeContent(w, r, re.Path(), time.Unix(0, 0), rs)
	})
	log.Infof("Serving Web at %v", addr)
	return http.Serve(ln, mux)
}

func (s *Web) Close() {
	if s.ln != nil {
		s.ln.Close()
	}
}
