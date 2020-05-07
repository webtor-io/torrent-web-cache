package services

import (
	"context"
	"sync"
	"time"
)

const (
	COMPLETED_PIECES_TTL = 10
)

type CompletedPiecesPool struct {
	sm     sync.Map
	timers sync.Map
	expire time.Duration
	st     *S3Storage
}

func NewCompletedPiecesPool(st *S3Storage) *CompletedPiecesPool {
	return &CompletedPiecesPool{expire: time.Duration(COMPLETED_PIECES_TTL) * time.Second, st: st}
}

func (s *CompletedPiecesPool) Get(ctx context.Context, h string) (*CompletedPieces, error) {
	v, _ := s.sm.LoadOrStore(h, NewCompletedPiecesLoader(ctx, h, s.st))
	t, tLoaded := s.timers.LoadOrStore(h, time.NewTimer(s.expire))
	timer := t.(*time.Timer)
	if !tLoaded {
		go func() {
			<-timer.C
			s.sm.Delete(h)
			s.timers.Delete(h)
		}()
	}
	return v.(*CompletedPiecesLoader).Get()
}
