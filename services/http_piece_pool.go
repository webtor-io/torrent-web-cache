package services

import (
	"io"
	"net/http"
	"sync"
)

type HTTPPiecePool struct {
	cl  *http.Client
	sm  sync.Map
	mux sync.Mutex
}

func NewHTTPPiecePool(cl *http.Client) *HTTPPiecePool {
	return &HTTPPiecePool{cl: cl}
}

func (s *HTTPPiecePool) Get(src string, h string, p string, q string) (io.ReadCloser, error) {
	v, loaded := s.sm.LoadOrStore(p, NewHTTPPieceLoader(s.cl, src, h, p, q))
	if !loaded {
		defer s.sm.Delete(p)
	}
	return v.(*HTTPPieceLoader).Get()
}
