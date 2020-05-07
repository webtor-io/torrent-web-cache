package services

import (
	"context"
	"io"
	"net/http"
)

type HTTPPiecePool struct {
	cl *http.Client
}

func NewHTTPPiecePool(cl *http.Client) *HTTPPiecePool {
	return &HTTPPiecePool{cl: cl}
}

func (s *HTTPPiecePool) Get(src string, h string, p string, q string, start int64, end int64, ctx context.Context) (io.ReadCloser, error) {
	l := NewHTTPPieceLoader(s.cl, src, h, p, q, start, end, ctx)
	return l.Get()
}
