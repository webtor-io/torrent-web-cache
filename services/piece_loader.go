package services

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/pkg/errors"
	"github.com/urfave/cli"

	log "github.com/sirupsen/logrus"
)

const (
	DATA_DIR = "data-dir"
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
	f      string
	err    error
	inited bool
	dd     string
}

func RegisterPieceLoaderFlags(c *cli.App) {
	c.Flags = append(c.Flags, cli.StringFlag{
		Name:   DATA_DIR,
		Usage:  "Data dir",
		Value:  "data",
		EnvVar: "DATA_DIR",
	})
}

func NewPieceLoader(c *cli.Context, cpp *CompletedPiecesPool, s3pp *S3PiecePool,
	httppp *HTTPPiecePool, src string, h string, p string, q string) *PieceLoader {
	return &PieceLoader{cpp: cpp, s3pp: s3pp, httppp: httppp, src: src, h: h, p: p, q: q, inited: false, dd: c.String(DATA_DIR)}
}

func (s *PieceLoader) Clear() {
	if s.f != "" {
		os.Remove(s.f)
		log.Infof("Piece removed path=%v", s.f)
	}
}

func (s *PieceLoader) Get() (*os.File, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if !s.inited {
		s.f, s.err = s.get()
		s.inited = true
	}
	if s.err != nil {
		return nil, s.err
	}
	return os.Open(s.f)
}

func (s *PieceLoader) get() (string, error) {
	cp, err := s.cpp.Get(s.h)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get Completed Pieces")
	}
	a, err := hex.DecodeString(s.p)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to decode hex hash=%v", s.p)
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
		return "", errors.Wrapf(err, "Failed to get piece hash=%v piece=%v", s.h, s.p)
	}
	defer r.Close()
	fn := fmt.Sprintf("%v/%v", s.dd, s.p)
	f, err := os.Create(fn)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to create file=%v", fn)
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to copy piece to file=%v", fn)
	}
	return fn, nil
}
