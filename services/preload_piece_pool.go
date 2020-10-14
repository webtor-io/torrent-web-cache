package services

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type PreloadReader struct {
	r    io.Reader
	clCh chan bool
}

type PreloadPiecePool struct {
	pp *PiecePool
	sm sync.Map
}

type PiecePreloader struct {
	pp     *PiecePool
	src    string
	h      string
	p      string
	q      string
	err    error
	inited bool
	b      []byte
	r      *PreloadReader
	ctx    context.Context
	mux    sync.Mutex
	clCh   chan bool
}

func NewPreloadReader(r io.Reader) *PreloadReader {
	return &PreloadReader{r: r, clCh: make(chan bool)}
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
	close(s.clCh)
	return nil
}

func NewPiecePreloader(ctx context.Context, pp *PiecePool, src string, h string, p string, q string) *PiecePreloader {
	return &PiecePreloader{ctx: ctx, pp: pp, src: src,
		h: h, p: p, q: q, clCh: make(chan bool)}
}

func (s *PiecePreloader) Preload() {
	if s.inited {
		return
	}
	s.mux.Lock()
	go func() {
		defer s.mux.Unlock()
		s.b, s.err = s.preload()
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
	buf := bytes.NewReader(s.b)
	if full {
		s.r = NewPreloadReader(buf)
	} else {
		_, err := buf.Seek(start, io.SeekStart)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to seek to %v in piece=%v", start, s.p)
		}
		lr := io.LimitReader(buf, end-start+1)
		s.r = NewPreloadReader(lr)
	}
	go func() {
		select {
		case <-s.r.clCh:
		case <-time.After(60 * time.Second):
		}
		close(s.clCh)
		log.Infof("Preloader closed hash=%v piece=%v", s.h, s.p)
	}()
	return s.r, nil
}

func (s *PiecePreloader) preload() ([]byte, error) {
	log.Infof("Start preloading hash=%v piece=%v", s.h, s.p)
	r, err := s.pp.Get(s.ctx, s.src, s.h, s.p, s.q, 0, 0, true)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to preload piece=%v", s.p)
	}
	return ioutil.ReadAll(r)
}

func NewPreloadPiecePool(pp *PiecePool) *PreloadPiecePool {
	return &PreloadPiecePool{pp: pp}
}

func (s *PreloadPiecePool) Get(ctx context.Context, src string, h string, p string, q string, start int64, end int64, full bool) (io.ReadCloser, error) {
	v, ok := s.sm.Load(p)
	if ok {
		s.sm.Delete(p)
		return v.(*PiecePreloader).Get(start, end, full)
	}
	return s.pp.Get(ctx, src, h, p, q, start, end, full)
}

func (s *PreloadPiecePool) Preload(ctx context.Context, src string, h string, p string, q string) {
	v, loaded := s.sm.LoadOrStore(p, NewPiecePreloader(ctx, s.pp, src, h, p, q))
	if !loaded {
		go func() {
			<-v.(*PiecePreloader).clCh
			s.sm.Delete(p)
		}()
		v.(*PiecePreloader).Preload()
	}
}
