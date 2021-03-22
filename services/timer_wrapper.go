package services

import (
	"time"
)

type TimerWrapper struct {
	t      *time.Timer
	d      time.Duration
	inited bool
}

func NewTimerWrapper(d time.Duration) *TimerWrapper {
	return &TimerWrapper{d: d}
}

func (s *TimerWrapper) Get() *time.Timer {
	if s.inited {
		return s.t
	}
	s.t = time.NewTimer(s.d)
	return s.t
}
