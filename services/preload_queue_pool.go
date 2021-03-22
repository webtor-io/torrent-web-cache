package services

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	PRELOAD_QUEUE_TTL = 60
)

type PreloadQueuePool struct {
	sm     sync.Map
	timers sync.Map
	pp     *PreloadPiecePool
	expire time.Duration
}

func NewPreloadQueuePool(pp *PreloadPiecePool) *PreloadQueuePool {
	return &PreloadQueuePool{
		pp:     pp,
		expire: time.Duration(PRELOAD_QUEUE_TTL) * time.Second,
	}
}

func (s *PreloadQueuePool) Push(key string, src string, h string, p string, q string) {
	v, _ := s.sm.LoadOrStore(key, NewPreloadQueue(s.pp))
	t, tLoaded := s.timers.LoadOrStore(key, NewTimerWrapper(s.expire))
	timer := t.(*TimerWrapper)
	if !tLoaded {
		log.Infof("New preload queue key=%v", key)
		go func(t *TimerWrapper) {
			<-t.Get().C
			log.Infof("Clean preload queue key=%v", key)
			s.sm.Delete(key)
			s.timers.Delete(key)
			v.(*PreloadQueue).Close()
		}(timer)
	} else {
		timer.Get().Reset(s.expire)
	}
	v.(*PreloadQueue).Push(src, h, p, q)
}
