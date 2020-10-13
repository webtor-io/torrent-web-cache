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
	ctx    context.Context
	mux    sync.Mutex
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
	defer s.mux.Unlock()
	s.b, s.err = s.preload()
	s.inited = true
}

func (s *PiecePreloader) Get(start int64, end int64) (io.ReadCloser, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	buf := bytes.NewBuffer(s.b)
	_, err := io.CopyN(ioutil.Discard, buf, start)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to skip %v bytes from piece=%v", start, s.p)
	}
	lr := io.LimitReader(buf, end-start)
	rcr := ioutil.NopCloser(lr)
	return rcr, nil
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
		log.Infof("Using preloaded piece hash=%v piece=%v", h, p)
		return v.(*PiecePreloader).Get(start, end)
	}
	return s.pp.Get(ctx, src, h, p, q, start, end, full)
}

func (s *PreloadPiecePool) Preload(ctx context.Context, src string, h string, p string, q string) {
	v, loaded := s.sm.LoadOrStore(p, NewPiecePreloader(ctx, s.pp, src, h, p, q))
	if !loaded {
		go func() {
			<-time.After(60 * time.Second)
			s.sm.Delete(p)
		}()
	} else {
		go v.(*PiecePreloader).Preload()
	}
}
