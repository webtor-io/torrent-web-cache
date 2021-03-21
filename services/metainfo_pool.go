package services

import (
	"context"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"
)

const (
	META_INFO_TTL = 60
)

type MetaInfoPool struct {
	sm     sync.Map
	timers sync.Map
	expire time.Duration
	st     *S3Storage
	mux    sync.Mutex
}

func NewMetaInfoPool(st *S3Storage) *MetaInfoPool {
	return &MetaInfoPool{expire: time.Duration(META_INFO_TTL) * time.Second, st: st}
}

func (s *MetaInfoPool) Get(h string) (*metainfo.Info, error) {
	v, _ := s.sm.LoadOrStore(h, NewMetaInfoLoader(context.Background(), h, s.st))
	t, tLoaded := s.timers.LoadOrStore(h, time.NewTimer(s.expire))
	timer := t.(*time.Timer)
	if !tLoaded {
		go func() {
			<-timer.C
			s.sm.Delete(h)
			s.timers.Delete(h)
		}()
	} else {
		s.mux.Lock()
		timer.Reset(s.expire)
		s.mux.Unlock()
	}

	return v.(*MetaInfoLoader).Get()
}
