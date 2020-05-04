package services

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type HTTPPieceLoader struct {
	cl     *http.Client
	src    string
	h      string
	p      string
	q      string
	mux    sync.Mutex
	r      io.ReadCloser
	err    error
	inited bool
}

func NewHTTPPieceLoader(cl *http.Client, src string, h string, p string, q string) *HTTPPieceLoader {
	return &HTTPPieceLoader{cl: cl, src: src, h: h, p: p, q: q, inited: false}
}

func (s *HTTPPieceLoader) Get() (io.ReadCloser, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.r, s.err
	}
	s.r, s.err = s.get()
	s.inited = true
	return s.r, s.err
}

func (s *HTTPPieceLoader) get() (io.ReadCloser, error) {
	t := time.Now()
	u := fmt.Sprintf("%v/%v/piece/%v", s.src, s.h, s.p)
	if s.q != "" {
		u = u + "?" + s.q
	}
	log.Infof("Start loading source piece src=%v", u)
	r, err := s.cl.Get(u)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to fetch torrent piece src=%v", u)
	}
	log.Infof("Finish loading source piece src=%v time=%v", u, time.Since(t))
	return r.Body, nil
}
