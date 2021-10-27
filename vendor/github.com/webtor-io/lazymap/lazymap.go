package lazymap

import (
	"math"
	"sort"
	"sync"
	"time"
)

type LazyMap struct {
	mux            sync.RWMutex
	m              map[string]*lazyMapItem
	expire         time.Duration
	errorExpire    time.Duration
	c              chan bool
	capacity       int
	cleanThreshold float64
	cleanRatio     float64
	cleaning       bool
}

type Config struct {
	Concurrency    int
	Expire         time.Duration
	ErrorExpire    time.Duration
	Capacity       int
	CleanThreshold float64
	CleanRatio     float64
}

func New(conf *Config) LazyMap {
	capacity := conf.Capacity
	concurrency := 10
	if conf.Concurrency != 0 {
		concurrency = conf.Concurrency
	}
	expire := conf.Expire
	errorExpire := expire
	if conf.ErrorExpire != 0 {
		errorExpire = conf.ErrorExpire
	}
	cleanThreshold := 0.9
	if conf.CleanThreshold != 0 {
		cleanThreshold = conf.CleanThreshold
	}
	cleanRatio := 0.1
	if conf.CleanRatio != 0 {
		cleanRatio = conf.CleanRatio
	}
	c := make(chan bool, concurrency)
	for i := 0; i < concurrency; i++ {
		c <- true
	}
	return LazyMap{
		c:              c,
		expire:         expire,
		errorExpire:    errorExpire,
		capacity:       capacity,
		cleanThreshold: cleanThreshold,
		cleanRatio:     cleanRatio,
		m:              make(map[string]*lazyMapItem, capacity),
	}
}

type lazyMapItem struct {
	key    string
	val    interface{}
	f      func() (interface{}, error)
	inited bool
	err    error
	la     time.Time
	mux    sync.Mutex
}

func (s *lazyMapItem) Get() (interface{}, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.la = time.Now()
	if s.inited {
		return s.val, s.err
	}
	s.val, s.err = s.f()
	s.inited = true
	return s.val, s.err
}

func (s *LazyMap) doExpire(expire time.Duration, key string) {
	<-time.After(expire)
	s.mux.Lock()
	delete(s.m, key)
	s.mux.Unlock()
}

func (s *LazyMap) clean() {
	if s.capacity == 0 {
		return
	}
	if len(s.m) < int(math.Ceil(s.cleanThreshold*float64(s.capacity))) {
		return
	}
	if s.cleaning {
		return
	}
	s.cleaning = true
	t := make([]*lazyMapItem, 0, len(s.m))
	for _, v := range s.m {
		t = append(t, v)
	}
	sort.Slice(t, func(i, j int) bool {
		return t[i].la.Before(t[j].la)
	})
	for i := 0; i < int(math.Ceil(s.cleanRatio*float64(s.capacity))); i++ {
		if t[i].inited {
			delete(s.m, t[i].key)
		}
	}
	s.cleaning = false
}

func (s *LazyMap) Has(key string) bool {
	// s.mux.RLock()
	// defer s.mux.RUnlock()
	_, loaded := s.m[key]
	return loaded
}

func (s *LazyMap) Get(key string, f func() (interface{}, error)) (interface{}, error) {
	s.mux.RLock()
	v, loaded := s.m[key]
	if loaded {
		s.mux.RUnlock()
		return v.Get()
	}
	s.mux.RUnlock()
	s.mux.Lock()
	v, loaded = s.m[key]
	if loaded {
		s.mux.Unlock()
		return v.Get()
	}
	<-s.c
	s.clean()
	v = &lazyMapItem{
		key: key,
		f:   f,
	}
	s.m[key] = v
	s.mux.Unlock()
	r, err := v.Get()
	if err != nil && s.errorExpire != 0 {
		go s.doExpire(s.errorExpire, key)
	} else if err == nil && s.expire != 0 {
		go s.doExpire(s.expire, key)
	}
	s.c <- true
	return r, err
}
