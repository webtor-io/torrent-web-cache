package services

import (
	"context"
	"sync"
	"time"
)

const (
	TORRENT_TOUCH_TTL = 600
)

type TorrentTouchPool struct {
	sm     sync.Map
	st     *S3Storage
	expire time.Duration
}

func NewTorrentTouchPool(st *S3Storage) *TorrentTouchPool {
	return &TorrentTouchPool{expire: time.Duration(TORRENT_TOUCH_TTL) * time.Second, st: st}
}

func (s *TorrentTouchPool) Touch(h string) error {
	_, loaded := s.sm.LoadOrStore(h, true)
	if !loaded {
		t := NewTorrentToucher(context.Background(), h, s.st)
		go func() {
			<-time.After(s.expire)
			s.sm.Delete(h)
		}()
		return t.Touch()
	}
	return nil
}
