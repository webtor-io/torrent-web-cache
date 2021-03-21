package services

import (
	"context"
	"sync"

	"github.com/pkg/errors"
)

type CompletedPiecesLoader struct {
	st       *S3Storage
	infoHash string
	mux      sync.Mutex
	cp       *CompletedPieces
	err      error
	inited   bool
	ctx      context.Context
}

func NewCompletedPiecesLoader(ctx context.Context, infoHash string, st *S3Storage) *CompletedPiecesLoader {
	return &CompletedPiecesLoader{ctx: ctx, st: st, infoHash: infoHash}
}

func (s *CompletedPiecesLoader) Get() (*CompletedPieces, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.cp, s.err
	}
	s.cp, s.err = s.get()
	s.inited = true
	return s.cp, s.err
}

func (s *CompletedPiecesLoader) get() (*CompletedPieces, error) {
	r, err := s.st.GetCompletedPieces(s.ctx, s.infoHash)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch completed pieces")
	}
	cp := &CompletedPieces{}
	if r != nil {
		defer r.Close()
		err = cp.Load(r)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to load completed pieces")
		}
	}
	return cp, nil
}
