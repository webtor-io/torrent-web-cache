package services

import (
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type S3PieceLoader struct {
	st        *S3Storage
	infoHash  string
	pieceHash string
	mux       sync.Mutex
	r         io.ReadCloser
	err       error
	inited    bool
}

func NewS3PieceLoader(infoHash string, pieceHash string, st *S3Storage) *S3PieceLoader {
	return &S3PieceLoader{st: st, infoHash: infoHash, pieceHash: pieceHash, inited: false}
}

func (s *S3PieceLoader) Get() (io.ReadCloser, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.r, s.err
	}
	s.r, s.err = s.get()
	s.inited = true
	return s.r, s.err
}

func (s *S3PieceLoader) get() (io.ReadCloser, error) {
	t := time.Now()
	p, err := s.st.GetPiece(s.infoHash, s.pieceHash)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to fetch s3 piece %v/%v", s.infoHash, s.pieceHash)
	}
	if p == nil {
		return nil, nil
	}
	log.Infof("Finish loading S3 piece infohash=%v piecehash=%v time=%v", s.infoHash, s.pieceHash, time.Since(t))
	return p, nil
}
