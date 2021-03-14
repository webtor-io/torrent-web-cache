package services

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"net/http/pprof"

	uu "net/url"

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
	cp   *CompletedPiecesPool
	rate string
}

const (
	WEB_HOST_FLAG  = "host"
	WEB_PORT_FLAG  = "port"
	WEB_SOURCE_URL = "source-url"
)

func NewWeb(c *cli.Context, rp *ReaderPool, cp *CompletedPiecesPool) *Web {
	return &Web{cp: cp, src: c.String(WEB_SOURCE_URL), host: c.String(WEB_HOST_FLAG), port: c.Int(WEB_PORT_FLAG), rp: rp}
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

func (s *Web) getSourceURL(r *http.Request) (string, error) {
	su := r.Header.Get("X-Source-Url")
	if s.src != "" {
		su = s.src
	}
	u, err := uu.Parse(su)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to parse source url=%v", su)
	}
	// u.Path = u.Path + strings.TrimPrefix(r.URL.Path, "/")
	return u.String(), nil
}

func (s *Web) addCORSHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func (s *Web) serveContent(w http.ResponseWriter, r *http.Request, piece string) {
	s.addCORSHeaders(w, r)

	url, err := s.getSourceURL(r)
	if err != nil {
		log.WithError(err).Errorf("Failed to get source url=%v", url)
		w.WriteHeader(500)
		return
	}

	download := true
	keys, ok := r.URL.Query()["download"]
	if !ok || len(keys[0]) < 1 {
		download = false
	}
	if download {
		downloadFile := piece
		if piece == "" {
			u, err := uu.Parse(url)
			if err != nil {
				log.WithError(err).Errorf("Failed to parse source url=%v", url)
				w.WriteHeader(500)
			}
			downloadFile = filepath.Base(u.Path)
		}
		w.Header().Add("Content-Type", "application/octet-stream")
		w.Header().Add("Content-Disposition", "attachment; filename=\""+downloadFile+"\"")
	}
	tr, u, p, err := s.rp.Get(r.Context(), url, piece)
	if u != "" {
		http.Redirect(w, r, u, 302)
		return
	}
	http.ServeContent(NewRWConnector(w), r, p, time.Unix(0, 0), tr)
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

	mux.HandleFunc("/completed_pieces", func(w http.ResponseWriter, r *http.Request) {
		s.addCORSHeaders(w, r)
		url, err := s.getSourceURL(r)
		if err != nil {
			log.WithError(err).Errorf("Failed to get source url=%v", url)
			w.WriteHeader(500)
			return
		}
		u, err := uu.Parse(url)
		if err != nil {
			log.WithError(err).Errorf("Failed to parse source url=%v", url)
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

	mux.HandleFunc("/piece/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/piece/")
		w.Header().Set("Content-Type", "application/octet-stream")
		s.serveContent(w, r, p)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.serveContent(w, r, "")
	})
	log.Infof("Serving Web at %v", addr)
	return http.Serve(ln, mux)
}

func (s *Web) Close() {
	if s.ln != nil {
		s.ln.Close()
	}
}
