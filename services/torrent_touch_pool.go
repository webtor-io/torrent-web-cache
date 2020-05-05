package services

import (
	"sync"
	"time"
)

const (
	TORRENT_TOUCH_TTL = 600
)

type TorrentTouchPool struct {
	sm     sync.Map
	timers sync.Map
	expire time.Duration
	st     *S3Storage
}

func NewTorrentTouchPool(st *S3Storage) *TorrentTouchPool {
	return &TorrentTouchPool{expire: time.Duration(TORRENT_TOUCH_TTL) * time.Second, st: st}
}

func (s *TorrentTouchPool) Touch(h string) error {
	v, loaded := s.sm.LoadOrStore(h, NewTorrentToucher(h, s.st))
	if !loaded {
		return v.(*TorrentToucher).Touch()
		go func() {
			<-time.After(s.expire)
			s.sm.Delete(h)
		}()
	}
	return nil
}
