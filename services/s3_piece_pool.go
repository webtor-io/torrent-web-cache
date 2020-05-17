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

func (s *S3PiecePool) Get(ctx context.Context, h string, p string, start int64, end int64, full bool) (io.ReadCloser, error) {
	l := NewS3PieceLoader(ctx, h, p, s.st, start, end, full)
	return l.Get()
}
