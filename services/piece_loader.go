package services

import (
	"encoding/hex"
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
	mux    sync.Mutex
	r      *ReaderWrapper
	err    error
	inited bool
	l      int64
}

func NewPieceLoader(cpp *CompletedPiecesPool, s3pp *S3PiecePool,
	httppp *HTTPPiecePool, src string, h string, p string, q string, l int64) *PieceLoader {
	return &PieceLoader{cpp: cpp, s3pp: s3pp, httppp: httppp, src: src, h: h, p: p, q: q, inited: false, l: l}
}

func (s *PieceLoader) Get() (*ReaderWrapper, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.r, s.err
	}
	s.r, s.err = s.get()
	s.inited = true
	return s.r, s.err
}

func (s *PieceLoader) get() (*ReaderWrapper, error) {
	cp, err := s.cpp.Get(s.h)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get Completed Pieces")
	}
	a, err := hex.DecodeString(s.p)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to decode hex hash=%v", s.p)
	}
	var r io.ReadCloser
	var aa [20]byte
	copy(aa[:20], a)
	if cp.Has(aa) {
		r, err = s.s3pp.Get(s.h, s.p)
		if r == nil || err != nil {
			r, err = s.httppp.Get(s.src, s.h, s.p, s.q)
		}
	} else {
		r, err = s.httppp.Get(s.src, s.h, s.p, s.q)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get piece hash=%v piece=%v", s.h, s.p)
	}
	return NewReaderWrapper(r, s.l), nil
}
