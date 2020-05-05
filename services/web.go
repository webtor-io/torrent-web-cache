package services

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"net/http/pprof"

	"code.cloudfoundry.org/bytefmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type Web struct {
	host string
	port int
	src  string
	ln   net.Listener
	rp   *ReaderPool
}

const (
	WEB_HOST_FLAG  = "host"
	WEB_PORT_FLAG  = "port"
	WEB_SOURCE_URL = "source-url"
)

func NewWeb(c *cli.Context, rp *ReaderPool) *Web {
	return &Web{src: c.String(WEB_SOURCE_URL), host: c.String(WEB_HOST_FLAG), port: c.Int(WEB_PORT_FLAG), rp: rp}
}

func RegisterWebFlags(c *cli.App) {
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:   WEB_SOURCE_URL,
		Usage:  "source url",
		Value:  "",
		EnvVar: "SOURCE_URL",
	})
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

func (s *Web) getSourceURL(r *http.Request) string {
	if s.src != "" {
		return s.src
	}
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

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url := s.getSourceURL(r)
		if url == "" {
			log.Error("No source url provided")
			w.WriteHeader(500)
			return
		}
		re, err := s.rp.Get(url)
		defer re.Close()
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
