package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"net/http/pprof"

	uu "net/url"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	rp "runtime/pprof"
)

type Web struct {
	host string
	port int
	src  string
	ln   net.Listener
	rp   *ReaderPool
	cp   *CompletedPiecesPool
	rate string
}

const (
	WEB_HOST_FLAG     = "host"
	WEB_PORT_FLAG     = "port"
	WEB_SOURCE_URL    = "source-url"
	WEB_DOWNLOAD_RATE = "download-rate"
)

func NewWeb(c *cli.Context, rp *ReaderPool, cp *CompletedPiecesPool) *Web {
	return &Web{cp: cp, rate: c.String(WEB_DOWNLOAD_RATE), src: c.String(WEB_SOURCE_URL), host: c.String(WEB_HOST_FLAG), port: c.Int(WEB_PORT_FLAG), rp: rp}
}

func RegisterWebFlags(c *cli.App) {
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:   WEB_SOURCE_URL,
		Usage:  "source url",
		Value:  "",
		EnvVar: "SOURCE_URL",
	})
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:   WEB_DOWNLOAD_RATE,
		Usage:  "download rate",
		Value:  "",
		EnvVar: "DOWNLOAD_RATE",
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

func (s *Web) getDownloadRate(r *http.Request) string {
	if s.rate != "" {
		return s.rate
	}
	return r.Header.Get("X-Download-Rate")
}

func (s *Web) getSourceURL(r *http.Request) string {
	if s.src != "" {
		return s.src + r.URL.Path
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
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	pctx := context.Background()

	// mux.HandleFunc("/debug/pprof/profile10", func(w http.ResponseWriter, r *http.Request) {
	// 	http.ServeFile(w, r, "cpu.prof")
	// })

	mux.HandleFunc("/completed_pieces", func(w http.ResponseWriter, r *http.Request) {
		url := s.getSourceURL(r)
		u, err := uu.Parse(url)
		if err != nil {
			log.WithError(err).Errorf("Failed get parse source url=%v", url)
			w.WriteHeader(500)
			return
		}
		parts := strings.SplitN(u.Path, "/", 3)
		hash := parts[1]
		cp, err := s.cp.Get(hash)
		if err != nil {
			log.WithError(err).Errorf("Failed get completed pieces hash=%v", hash)
			w.WriteHeader(500)
			return
		}
		_, err = w.Write(cp.ToBytes())
		if err != nil {
			log.WithError(err).Errorf("Failed to write completed pieces hash=%v", hash)
			w.WriteHeader(500)
			return
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		url := s.getSourceURL(r)
		if url == "" {
			log.Error("No source url provided")
			w.WriteHeader(500)
			return
		}
		re, err := s.rp.Get(r.Context(), url, s.getDownloadRate(r))
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
		u, _ := uu.Parse(url)
		labels := rp.Labels("path", u.Path)
		rp.Do(pctx, labels, func(ctx context.Context) {
			http.ServeContent(NewRWConnector(w), r, re.Path(), time.Unix(0, 0), re)
		})
	})
	log.Infof("Serving Web at %v", addr)
	return http.Serve(ln, mux)
}

func (s *Web) Close() {
	if s.ln != nil {
		s.ln.Close()
	}
}
