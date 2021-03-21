package services

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	PRELOAD_TTL            = 10
	PRELOAD_CACHE_PATH     = "cache"
	PRELOAD_CACHE_MAX_SIZE = 10_000_000_000
)

type PreloadReader struct {
	r      io.Reader
	f      *os.File
	h      string
	p      string
	closed bool
}

type PreloadPiecePool struct {
	pp       *PiecePool
	sm       sync.Map
	timers   sync.Map
	expire   time.Duration
	inited   bool
	cleaning bool
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

func NewPreloadReader(f *os.File, r io.Reader, h string, p string) *PreloadReader {
	return &PreloadReader{f: f, r: r, h: h, p: p}
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
	if s.closed {
		return nil
	}
	s.closed = true
	log.Infof("Closing reader hash=%v piece=%v", s.h, s.p)
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
	s.inited = true
	defer s.mux.Unlock()
	s.err = s.preload()
}

func (s *PiecePreloader) Get(start int64, end int64, full bool) (io.ReadCloser, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	log.Infof("Using preloaded piece hash=%v piece=%v, start=%v end=%v full=%v", s.h, s.p, start, end, full)
	if s.err != nil {
		return nil, s.err
	}
	path := PRELOAD_CACHE_PATH + "/" + s.p
	f, err := os.Open(path)
	if err != nil {
		return nil, s.err
	}
	if full {
		return NewPreloadReader(f, f, s.h, s.p), nil
	} else {
		_, err := f.Seek(start, io.SeekStart)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to seek to %v in piece=%v", start, s.p)
		}
		lr := io.LimitReader(f, end-start+1)
		return NewPreloadReader(f, lr, s.h, s.p), nil
	}
}
func (s *PiecePreloader) Clean() error {
	// s.mux.Lock()
	// defer s.mux.Unlock()
	// path := PRELOAD_CACHE_PATH + "/" + s.p
	// return os.Remove(path)
	return nil
}

func (s *PiecePreloader) preload() error {
	path := PRELOAD_CACHE_PATH + "/" + s.p
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Infof("Start preloading hash=%v piece=%v", s.h, s.p)
		r, err := s.pp.Get(s.ctx, s.src, s.h, s.p, s.q, 0, 0, true)
		if err != nil {
			errors.Wrapf(err, "Failed to preload piece=%v", s.p)
		}
		f, err := os.Create(path)
		if err != nil {
			return errors.Wrapf(err, "Failed to create preload file piece=%v path=%v", s.p, path)
		}
		_, err = io.Copy(f, r)
		return err
	} else {
		log.Infof("Preload data already exists hash=%v piece=%v", s.h, s.p)
		return nil
	}
}

func NewPreloadPiecePool(pp *PiecePool) *PreloadPiecePool {
	return &PreloadPiecePool{pp: pp, expire: time.Duration(PRELOAD_TTL) * time.Second}
}

func (s *PreloadPiecePool) Get(ctx context.Context, src string, h string, p string, q string, start int64, end int64, full bool) (io.ReadCloser, error) {
	v, ok := s.sm.Load(p)
	if ok {
		tt, ok := s.timers.Load(p)
		if ok {
			tt.(*time.Timer).Reset(s.expire)
		}
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
func (s *PreloadPiecePool) cleanCache() error {
	if s.cleaning {
		return nil
	}
	s.cleaning = true
	defer func() {
		s.cleaning = false
	}()
	var size int64
	err := filepath.Walk(PRELOAD_CACHE_PATH, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	if err != nil {
		return err
	}
	if size < PRELOAD_CACHE_MAX_SIZE {
		return nil
	}
	files, err := ioutil.ReadDir(PRELOAD_CACHE_PATH)
	if err != nil {
		return err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().Before(files[j].ModTime())
	})
	for _, f := range files {
		if _, ok := s.sm.Load(f.Name()); ok {
			continue
		}
		path := PRELOAD_CACHE_PATH + "/" + f.Name()
		err := os.Remove(path)
		if err != nil {
			return err
		}
		log.Infof("Clean cache file name=%v time=%v size=%v", f.Name(), f.ModTime(), f.Size())
		size = size - f.Size()
		if size < PRELOAD_CACHE_MAX_SIZE {
			return nil
		}
	}
	return nil
}
func (s *PreloadPiecePool) Preload(src string, h string, p string, q string) {
	if !s.inited {
		err := os.MkdirAll(PRELOAD_CACHE_PATH, 0777)
		if err != nil {
			log.WithError(err).Warnf("Failed to create cache folder path=%v", PRELOAD_CACHE_PATH)
		}
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			for range ticker.C {
				go func() {
					err := s.cleanCache()
					if err != nil {
						log.WithError(err).Warnf("Failed to clean cache folder path=%v", PRELOAD_CACHE_PATH)
					}
				}()
			}
		}()
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
