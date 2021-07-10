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

	"code.cloudfoundry.org/bytefmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	PRELOAD_TTL                      = 10
	PRELOAD_TIMEOUT                  = 60
	PRELOAD_CACHE_PATH               = "cache"
	PRELOAD_CLEAR_CACHE_ON_EXIT_FLAG = "preload-clear-cache-on-exit"
	PRELOAD_CACHE_SIZE_FLAG          = "preload-cache-size"
)

func RegisterPreloadFlags(c *cli.App) {
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:   PRELOAD_CACHE_SIZE_FLAG,
		Usage:  "preload cache size",
		Value:  "10G",
		EnvVar: "PRELOAD_CACHE_SIZE",
	})
	c.Flags = append(c.Flags, cli.BoolTFlag{
		Name:   PRELOAD_CLEAR_CACHE_ON_EXIT_FLAG,
		Usage:  "preload clear cache on exit",
		EnvVar: "PRELOAD_CLEAR_CACHE_ON_EXIT",
	})
}

type PreloadReader struct {
	r      io.Reader
	f      *os.File
	h      string
	p      string
	lb     *LeakyBuffer
	closed bool
}

type PreloadPiecePool struct {
	pp               *PiecePool
	sm               sync.Map
	timers           sync.Map
	expire           time.Duration
	lb               *LeakyBuffer
	inited           bool
	cleaning         bool
	cacheSize        uint64
	clearCacheOnExit bool
}

type PiecePreloader struct {
	pp     *PiecePool
	src    string
	h      string
	p      string
	q      string
	err    error
	inited bool
	lb     *LeakyBuffer
	ctx    context.Context
	mux    sync.Mutex
}

func NewPreloadReader(f *os.File, r io.Reader, lb *LeakyBuffer, h string, p string) *PreloadReader {
	return &PreloadReader{f: f, r: r, h: h, p: p, lb: lb}
}

func (s *PreloadReader) Read(p []byte) (n int, err error) {
	return s.r.Read(p)
}

func (r *PreloadReader) WriteTo(w io.Writer) (n int64, err error) {
	if l, ok := r.r.(io.WriterTo); ok {
		return l.WriteTo(w)
	}
	buf := r.lb.Get()
	n, err = io.CopyBuffer(w, r.r, buf)
	r.lb.Put(buf)
	return
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

func NewPiecePreloader(ctx context.Context, pp *PiecePool, lb *LeakyBuffer, src string, h string, p string, q string) *PiecePreloader {
	return &PiecePreloader{ctx: ctx, pp: pp, src: src,
		h: h, p: p, q: q, lb: lb}
}

func (s *PiecePreloader) Preload() error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.err
	}
	s.err = s.preload()
	s.inited = true
	return s.err
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
		return NewPreloadReader(f, f, s.lb, s.h, s.p), nil
	} else {
		_, err := f.Seek(start, io.SeekStart)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to seek to %v in piece=%v", start, s.p)
		}
		lr := io.LimitReader(f, end-start+1)
		return NewPreloadReader(f, lr, s.lb, s.h, s.p), nil
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
	tempPath := PRELOAD_CACHE_PATH + "/_" + s.p
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Infof("Start preloading hash=%v piece=%v", s.h, s.p)
		r, err := s.pp.Get(s.ctx, s.src, s.h, s.p, s.q, 0, 0, true)
		if err != nil {
			return errors.Wrapf(err, "Failed to preload piece=%v", s.p)
		}
		f, err := os.Create(tempPath)
		if err != nil {
			return errors.Wrapf(err, "Failed to create preload file piece=%v path=%v", s.p, tempPath)
		}
		buf := s.lb.Get()
		_, err = io.CopyBuffer(f, r, buf)
		s.lb.Put(buf)
		if err != nil {
			return errors.Wrapf(err, "Failed to write preload file piece=%v path=%v", s.p, tempPath)
		}
		err = os.Rename(tempPath, path)
		if err != nil {
			return errors.Wrapf(err, "Failed to rename file from=%v to=%v", tempPath, path)
		}
		return nil
	} else {
		t := time.Now().Local()
		err := os.Chtimes(path, t, t)
		if err != nil {
			return errors.Wrapf(err, "Failed to change preload file modification date piece=%v path=%v", s.p, path)
		}
		log.Infof("Preload data already exists hash=%v piece=%v", s.h, s.p)
		return nil
	}
}

func NewPreloadPiecePool(c *cli.Context, pp *PiecePool, lb *LeakyBuffer) (*PreloadPiecePool, error) {
	pcs, err := bytefmt.ToBytes(c.String(PRELOAD_CACHE_SIZE_FLAG))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse preload cache size %v", c.String(PRELOAD_CACHE_SIZE_FLAG))
	}
	return &PreloadPiecePool{
		pp:               pp,
		lb:               lb,
		expire:           time.Duration(PRELOAD_TTL) * time.Second,
		clearCacheOnExit: c.Bool(PRELOAD_CLEAR_CACHE_ON_EXIT_FLAG),
		cacheSize:        pcs,
	}, nil
}

func (s *PreloadPiecePool) Get(ctx context.Context, src string, h string, p string, q string, start int64, end int64, full bool) (io.ReadCloser, error) {
	path := PRELOAD_CACHE_PATH + "/" + p
	exists := true
	if _, err := os.Stat(path); os.IsNotExist(err) {
		exists = false
	}
	if exists {
		s.Preload(src, h, p, q)
	}
	v, ok := s.sm.Load(p)
	if ok {
		tt, ok := s.timers.Load(p)
		if ok {
			tt.(*TimerWrapper).Get().Reset(s.expire)
		}
		return v.(*PiecePreloader).Get(start, end, full)
	}
	return s.pp.Get(ctx, src, h, p, q, start, end, full)
}
func (s *PreloadPiecePool) Close() {
	if !s.clearCacheOnExit {
		return
	}
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
	var size uint64
	err := filepath.Walk(PRELOAD_CACHE_PATH, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += uint64(info.Size())
		}
		return err
	})
	if err != nil {
		return err
	}
	if size < s.cacheSize {
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
			log.WithError(err).Warnf("Failed to clean cache file name=%v time=%v size=%v", f.Name(), f.ModTime(), f.Size())
		} else {
			log.Infof("Clean cache file name=%v time=%v size=%v", f.Name(), f.ModTime(), f.Size())
		}
		size = size - uint64(f.Size())
		if size < s.cacheSize {
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
			ticker := time.NewTicker(time.Duration(PRELOAD_TIMEOUT) * time.Second)
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
	pCtx, pC := context.WithTimeout(context.Background(), 1*time.Minute)
	defer pC()
	v, _ := s.sm.LoadOrStore(p, NewPiecePreloader(pCtx, s.pp, s.lb, src, h, p, q))
	t, tLoaded := s.timers.LoadOrStore(p, NewTimerWrapper(s.expire))
	timer := t.(*TimerWrapper)
	if !tLoaded {
		go func(t *TimerWrapper) {
			<-t.Get().C
			log.Infof("Clean preloaded piece hash=%v piece=%v", h, p)
			s.sm.Delete(p)
			s.timers.Delete(p)
			err := v.(*PiecePreloader).Clean()
			if err != nil {
				log.WithError(err).Warnf("Failed to clean preloaded piece hash=%v piece=%v", h, p)
			}
		}(timer)
		err := v.(*PiecePreloader).Preload()
		if err != nil {
			s.sm.Delete(p)
			s.timers.Delete(p)
			log.WithError(err).Warnf("Failed to preload piece hash=%v piece=%v", h, p)
		}
	} else {
		timer.Get().Reset(s.expire)
	}
}
