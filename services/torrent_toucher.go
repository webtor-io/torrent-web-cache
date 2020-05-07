package services

import (
	"context"
	"sync"
)

type TorrentToucher struct {
	st       *S3Storage
	infoHash string
	mux      sync.Mutex
	err      error
	inited   bool
	ctx      context.Context
}

func NewTorrentToucher(ctx context.Context, infoHash string, st *S3Storage) *TorrentToucher {
	return &TorrentToucher{st: st, infoHash: infoHash, inited: false, ctx: ctx}
}

func (s *TorrentToucher) Touch() error {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.err
	}
	s.err = s.touch()
	s.inited = true
	return s.err
}

func (s *TorrentToucher) touch() (err error) {
	err = s.st.TouchTorrent(s.ctx, s.infoHash)
	return
}
