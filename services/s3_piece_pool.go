package services

import (
	"context"
	"io"
)

type S3PiecePool struct {
	st *S3Storage
}

func NewS3PiecePool(st *S3Storage) *S3PiecePool {
	return &S3PiecePool{st: st}
}

func (s *S3PiecePool) Get(h string, p string, start int64, end int64, ctx context.Context) (io.ReadCloser, error) {
	l := NewS3PieceLoader(h, p, s.st, start, end, ctx)
	return l.Get()
}
