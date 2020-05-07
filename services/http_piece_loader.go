package services

import (
	"context"
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
	start  int64
	end    int64
	ctx    context.Context
}

func NewHTTPPieceLoader(ctx context.Context, cl *http.Client, src string, h string, p string, q string, start int64, end int64) *HTTPPieceLoader {
	return &HTTPPieceLoader{cl: cl, src: src, h: h, p: p, q: q, inited: false, start: start, end: end, ctx: ctx}
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
	ra := fmt.Sprintf("bytes=%v-%v", s.start, s.end)
	log.Debugf("Start loading source piece src=%v range=%v", u, ra)
	req, _ := http.NewRequestWithContext(s.ctx, "GET", u, nil)
	req.Header.Set("Range", ra)
	r, err := s.cl.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to fetch torrent piece src=%v", u)
	}
	log.Debugf("Finish loading source piece src=%v range=%v time=%v", u, ra, time.Since(t))
	return r.Body, nil
}
