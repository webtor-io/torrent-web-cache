package services

import (
	"net/http"
	"sync"
	"time"
)

const (
	HTTP_PIECE_TTL = 10
)

type HTTPPiecePool struct {
	cl     *http.Client
	sm     sync.Map
	timers sync.Map
	expire time.Duration
	mux    sync.Mutex
}

func NewHTTPPiecePool(cl *http.Client) *HTTPPiecePool {
	return &HTTPPiecePool{cl: cl, expire: time.Duration(HTTP_PIECE_TTL) * time.Second}
}

func (s *HTTPPiecePool) Get(src string, h string, p string, q string) ([]byte, error) {
	key := p
	v, _ := s.sm.LoadOrStore(key, NewHTTPPieceLoader(s.cl, src, h, p, q))
	t, tLoaded := s.timers.LoadOrStore(key, time.NewTimer(s.expire))
	timer := t.(*time.Timer)
	if !tLoaded {
		go func() {
			<-timer.C
			s.sm.Delete(key)
			s.timers.Delete(key)
		}()
	} else {
		s.mux.Lock()
		timer.Reset(s.expire)
		s.mux.Unlock()
	}

	return v.(*HTTPPieceLoader).Get()
}
