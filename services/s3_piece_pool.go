package services

import (
	"io"
	"sync"
)

type S3PiecePool struct {
	sm  sync.Map
	st  *S3Storage
	mux sync.Mutex
}

func NewS3PiecePool(st *S3Storage) *S3PiecePool {
	return &S3PiecePool{st: st}
}

func (s *S3PiecePool) Get(h string, p string) (io.ReadCloser, error) {
	v, loaded := s.sm.LoadOrStore(p, NewS3PieceLoader(h, p, s.st))
	if !loaded {
		defer s.sm.Delete(p)
	}
	return v.(*S3PieceLoader).Get()
}
