package services

import (
	"io"
	"net/http"
)

type HTTPPiecePool struct {
	cl *http.Client
}

func NewHTTPPiecePool(cl *http.Client) *HTTPPiecePool {
	return &HTTPPiecePool{cl: cl}
}

func (s *HTTPPiecePool) Get(src string, h string, p string, q string) (io.ReadCloser, error) {
	l := NewHTTPPieceLoader(s.cl, src, h, p, q)
	return l.Get()
}
