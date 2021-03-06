package services

import (
	"context"
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
	start     int64
	end       int64
	full      bool
	ctx       context.Context
}

func NewS3PieceLoader(ctx context.Context, infoHash string, pieceHash string, st *S3Storage, start int64, end int64, full bool) *S3PieceLoader {
	return &S3PieceLoader{st: st, infoHash: infoHash, pieceHash: pieceHash, inited: false, start: start, end: end, ctx: ctx, full: full}
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
	p, err := s.st.GetPiece(s.ctx, s.infoHash, s.pieceHash, s.start, s.end, s.full)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to fetch s3 piece %v/%v", s.infoHash, s.pieceHash)
	}
	if p == nil {
		return nil, nil
	}
	log.Debugf("Finish loading S3 piece infohash=%v piecehash=%v time=%v", s.infoHash, s.pieceHash, time.Since(t))
	return p, nil
}
