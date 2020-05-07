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

func (s *HTTPPiecePool) Get(ctx context.Context, src string, h string, p string, q string, start int64, end int64) (io.ReadCloser, error) {
	l := NewHTTPPieceLoader(ctx, s.cl, src, h, p, q, start, end)
	return l.Get()
}
