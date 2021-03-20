package services

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	PRELOAD_TTL        = 300
	PRELOAD_CACHE_PATH = "cache"
)

type PreloadReader struct {
	r io.Reader
	f *os.File
}

type PreloadPiecePool struct {
	pp     *PiecePool
	sm     sync.Map
	timers sync.Map
	expire time.Duration
	inited bool
}

type PiecePreloader struct {
	pp     *PiecePool
	src    string
	h      string
	p      string
	q      string
	err    error
	inited bool
	ctx    context.Context
	mux    sync.Mutex
}

func NewPreloadReader(f *os.File, r io.Reader) *PreloadReader {
	return &PreloadReader{f: f, r: r}
}

func (s *PreloadReader) Read(p []byte) (n int, err error) {
	return s.r.Read(p)
}

func (r *PreloadReader) WriteTo(w io.Writer) (n int64, err error) {
	if l, ok := r.r.(io.WriterTo); ok {
		return l.WriteTo(w)
	}
	return io.Copy(w, r.r)
}

func (s *PreloadReader) Close() error {
	s.f.Close()
	return nil
}

func NewPiecePreloader(ctx context.Context, pp *PiecePool, src string, h string, p string, q string) *PiecePreloader {
	return &PiecePreloader{ctx: ctx, pp: pp, src: src,
		h: h, p: p, q: q}
}

func (s *PiecePreloader) Preload() {
	if s.inited {
		return
	}
	s.mux.Lock()
	go func() {
		defer s.mux.Unlock()
		s.err = s.preload()
		s.inited = true
	}()
}

func (s *PiecePreloader) Get(start int64, end int64, full bool) (io.ReadCloser, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	log.Infof("Using preloaded piece hash=%v piece=%v", s.h, s.p)
	if s.err != nil {
		return nil, s.err
	}
	path := PRELOAD_CACHE_PATH + "/" + s.p
	f, err := os.Open(path)
	if err != nil {
		return nil, s.err
	}
	if full {
		return f, nil
	} else {
		_, err := f.Seek(start, io.SeekStart)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to seek to %v in piece=%v", start, s.p)
		}
		// lr := io.LimitReader(f, end-start+1)
		// return NewPreloadReader(f, lr), nil
		return f, nil
	}
}
func (s *PiecePreloader) Clean() error {
	path := PRELOAD_CACHE_PATH + "/" + s.p
	return os.Remove(path)
}

func (s *PiecePreloader) preload() error {
	log.Infof("Start preloading hash=%v piece=%v", s.h, s.p)
	r, err := s.pp.Get(s.ctx, s.src, s.h, s.p, s.q, 0, 0, true)
	if err != nil {
		errors.Wrapf(err, "Failed to preload piece=%v", s.p)
	}
	path := PRELOAD_CACHE_PATH + "/" + s.p
	f, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "Failed to create preload file piece=%v path=%v", s.p, path)
	}
	_, err = io.Copy(f, r)
	return err
}

func NewPreloadPiecePool(pp *PiecePool) *PreloadPiecePool {
	return &PreloadPiecePool{pp: pp, expire: time.Duration(PRELOAD_TTL) * time.Second}
}

func (s *PreloadPiecePool) Get(ctx context.Context, src string, h string, p string, q string, start int64, end int64, full bool) (io.ReadCloser, error) {
	v, ok := s.sm.Load(p)
	if ok {
		return v.(*PiecePreloader).Get(start, end, full)
	}
	return s.pp.Get(ctx, src, h, p, q, start, end, full)
}
func (s *PreloadPiecePool) Close() {
	err := os.RemoveAll(PRELOAD_CACHE_PATH)
	if err != nil {
		log.WithError(err).Warnf("Failed to clean cache folder path=%v", PRELOAD_CACHE_PATH)
	}
}

func (s *PreloadPiecePool) Preload(src string, h string, p string, q string) {
	if !s.inited {
		err := os.MkdirAll(PRELOAD_CACHE_PATH, 0777)
		if err != nil {
			log.WithError(err).Warnf("Failed to create cache folder path=%v", PRELOAD_CACHE_PATH)
		}
		s.inited = true
	}
	v, _ := s.sm.LoadOrStore(p, NewPiecePreloader(context.Background(), s.pp, src, h, p, q))
	t, tLoaded := s.timers.LoadOrStore(p, time.NewTimer(s.expire))
	timer := t.(*time.Timer)
	if !tLoaded {
		go func() {
			<-timer.C
			log.Infof("Clean preloaded piece hash=%v piece=%v", h, p)
			s.sm.Delete(p)
			s.timers.Delete(p)
			err := v.(*PiecePreloader).Clean()
			if err != nil {
				log.WithError(err).Warnf("Failed to clean preloaded piece hash=%v piece=%v", h, p)
			}
		}()
		v.(*PiecePreloader).Preload()
	} else {
		timer.Reset(s.expire)
	}
}
