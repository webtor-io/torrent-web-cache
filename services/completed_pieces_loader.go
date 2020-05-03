package services

import (
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
}

func NewCompletedPiecesLoader(infoHash string, st *S3Storage) *CompletedPiecesLoader {
	return &CompletedPiecesLoader{st: st, infoHash: infoHash, inited: false}
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
	r, err := s.st.GetCompletedPieces(s.infoHash)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch completed pieces")
	}
	defer r.Close()
	cp := &CompletedPieces{}
	if r != nil {
		err = cp.Load(r)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to load completed pieces")
		}
	}
	return cp, nil
}
