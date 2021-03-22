package services

import (
	"sync"
	"time"
)

type TimerWrapper struct {
	t      *time.Timer
	d      time.Duration
	inited bool
	mux    sync.Mutex
}

func NewTimerWrapper(d time.Duration) *TimerWrapper {
	return &TimerWrapper{d: d}
}

func (s *TimerWrapper) Get() *time.Timer {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.t
	}
	s.inited = true
	s.t = time.NewTimer(s.d)
	return s.t
}
