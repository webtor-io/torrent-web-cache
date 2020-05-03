package services

import (
	"sync"
)

type TorrentToucher struct {
	st       *S3Storage
	infoHash string
	mux      sync.Mutex
	err      error
	inited   bool
}

func NewTorrentToucher(infoHash string, st *S3Storage) *TorrentToucher {
	return &TorrentToucher{st: st, infoHash: infoHash, inited: false}
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
	err = s.st.TouchTorrent(s.infoHash)
	return
}
