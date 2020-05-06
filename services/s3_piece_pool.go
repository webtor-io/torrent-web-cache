package services

import (
	"io"
)

type S3PiecePool struct {
	st *S3Storage
}

func NewS3PiecePool(st *S3Storage) *S3PiecePool {
	return &S3PiecePool{st: st}
}

func (s *S3PiecePool) Get(h string, p string) (io.ReadCloser, error) {
	l := NewS3PieceLoader(h, p, s.st)
	return l.Get()
}
