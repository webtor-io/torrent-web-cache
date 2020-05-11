package services

import (
	"context"
	"io"
	"sync"

	"github.com/pkg/errors"
)

type PieceLoader struct {
	s3pp   *S3PiecePool
	httppp *HTTPPiecePool
	cpp    *CompletedPiecesPool
	src    string
	h      string
	p      string
	q      string
	start  int64
	end    int64
	mux    sync.Mutex
	r      io.ReadCloser
	err    error
	inited bool
	ctx    context.Context
}

func NewPieceLoader(ctx context.Context, cpp *CompletedPiecesPool, s3pp *S3PiecePool,
	httppp *HTTPPiecePool, src string, h string, p string, q string, start int64, end int64) *PieceLoader {
	return &PieceLoader{cpp: cpp, s3pp: s3pp, httppp: httppp, src: src, h: h, p: p, q: q, inited: false, start: start, end: end, ctx: ctx}
}

func (s *PieceLoader) Get() (io.ReadCloser, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.r, s.err
	}
	s.r, s.err = s.get()
	s.inited = true
	return s.r, s.err
}

func (s *PieceLoader) get() (io.ReadCloser, error) {
	cp, err := s.cpp.Get(s.h)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get Completed Pieces")
	}
	var r io.ReadCloser
	ok, err := cp.HasHex(s.p)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to check piece")
	}
	if ok {
		r, err = s.s3pp.Get(s.ctx, s.h, s.p, s.start, s.end)
		if r == nil || err != nil {
			r, err = s.httppp.Get(s.ctx, s.src, s.h, s.p, s.q, s.start, s.end)
		}
	} else {
		r, err = s.httppp.Get(s.ctx, s.src, s.h, s.p, s.q, s.start, s.end)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get piece hash=%v piece=%v", s.h, s.p)
	}
	return r, nil
}
