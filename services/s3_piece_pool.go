package services

import (
	"sync"
	"time"
)

const (
	S3_PIECE_TTL = 60
)

type S3PiecePool struct {
	sm     sync.Map
	timers sync.Map
	expire time.Duration
	st     *S3Storage
	mux    sync.Mutex
}

func NewS3PiecePool(st *S3Storage) *S3PiecePool {
	return &S3PiecePool{expire: time.Duration(S3_PIECE_TTL) * time.Second, st: st}
}

func (s *S3PiecePool) Get(h string, p string) ([]byte, error) {
	key := p
	v, _ := s.sm.LoadOrStore(key, NewS3PieceLoader(h, p, s.st))
	t, tLoaded := s.timers.LoadOrStore(key, time.NewTimer(s.expire))
	timer := t.(*time.Timer)
	if !tLoaded {
		go func() {
			<-timer.C
			s.sm.Delete(key)
			s.timers.Delete(key)
			v.(*S3PieceLoader).Clear()
		}()
	} else {
		s.mux.Lock()
		timer.Reset(s.expire)
		s.mux.Unlock()
	}

	return v.(*S3PieceLoader).Get()
}
