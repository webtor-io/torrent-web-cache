package services

import (
	"context"
	"sync"
)

type TorrentTouchPool struct {
	sm sync.Map
	st *S3Storage
}

func NewTorrentTouchPool(st *S3Storage) *TorrentTouchPool {
	return &TorrentTouchPool{st: st}
}

func (s *TorrentTouchPool) Touch(ctx context.Context, h string) error {
	_, loaded := s.sm.LoadOrStore(h, true)
	if !loaded {
		t := NewTorrentToucher(ctx, h, s.st)
		return t.Touch()
		go func() {
			<-ctx.Done()
			s.sm.Delete(h)
		}()
	}
	return nil
}
