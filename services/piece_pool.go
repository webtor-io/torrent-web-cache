package services

import (
	"io"
	"sync"
)

type PiecePool struct {
	s3pp   *S3PiecePool
	httppp *HTTPPiecePool
	cpp    *CompletedPiecesPool
	sm     sync.Map
}

func NewPiecePool(cpp *CompletedPiecesPool, s3pp *S3PiecePool,
	httppp *HTTPPiecePool) *PiecePool {
	return &PiecePool{s3pp: s3pp, httppp: httppp, cpp: cpp}
}

func (s *PiecePool) Get(src string, h string, p string, q string) (io.ReadSeeker, error) {
	v, loaded := s.sm.LoadOrStore(p, NewPieceLoader(s.cpp, s.s3pp, s.httppp, src, h, p, q))
	if !loaded {
		defer s.sm.Delete(p)
	}
	return v.(*PieceLoader).Get()
}
