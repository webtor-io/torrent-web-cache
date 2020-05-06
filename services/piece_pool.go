package services

import (
	"io"
	"sync"
	"time"
)

const (
	PIECE_TTL = 30
)

type PiecePool struct {
	s3pp   *S3PiecePool
	httppp *HTTPPiecePool
	cpp    *CompletedPiecesPool
	sm     sync.Map
	mux    sync.Mutex
	timers sync.Map
	expire time.Duration
}

func NewPiecePool(cpp *CompletedPiecesPool, s3pp *S3PiecePool,
	httppp *HTTPPiecePool) *PiecePool {
	return &PiecePool{s3pp: s3pp, httppp: httppp, cpp: cpp, expire: time.Duration(PIECE_TTL) * time.Second}
}

func (s *PiecePool) Get(src string, h string, p string, q string) (io.ReadSeeker, error) {
	v, _ := s.sm.LoadOrStore(p, NewPieceLoader(s.cpp, s.s3pp, s.httppp, src, h, p, q))
	t, tLoaded := s.timers.LoadOrStore(p, time.NewTimer(s.expire))
	timer := t.(*time.Timer)
	if !tLoaded {
		go func() {
			<-timer.C
			s.sm.Delete(p)
			s.timers.Delete(p)
			v.(*PieceLoader).Clear()
		}()
	} else {
		s.mux.Lock()
		timer.Reset(s.expire)
		s.mux.Unlock()
	}

	return v.(*PieceLoader).Get()
}
