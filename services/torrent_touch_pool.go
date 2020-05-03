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

func (s *TorrentTouchPool) Touch(h string) (err error) {
	v, _ := s.sm.LoadOrStore(h, NewTorrentToucher(h, s.st))
	t, tLoaded := s.timers.LoadOrStore(h, time.NewTimer(s.expire))
	timer := t.(*time.Timer)
	if !tLoaded {
		go func() {
			<-timer.C
			s.sm.Delete(h)
			s.timers.Delete(h)
		}()
	}
	err = v.(*TorrentToucher).Touch()
	return
}
