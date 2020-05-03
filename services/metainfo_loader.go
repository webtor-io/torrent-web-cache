package services

import (
	"sync"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/pkg/errors"
)

type MetaInfoLoader struct {
	st       *S3Storage
	infoHash string
	mux      sync.Mutex
	mi       *metainfo.MetaInfo
	err      error
	inited   bool
}

func NewMetaInfoLoader(infoHash string, st *S3Storage) *MetaInfoLoader {
	return &MetaInfoLoader{st: st, infoHash: infoHash, inited: false}
}

func (s *MetaInfoLoader) Get() (*metainfo.MetaInfo, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.mi, s.err
	}
	s.mi, s.err = s.get()
	s.inited = true
	return s.mi, s.err
}

func (s *MetaInfoLoader) get() (*metainfo.MetaInfo, error) {
	r, err := s.st.GetTorrent(s.infoHash)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch torrent")
	}
	defer r.Close()
	if r == nil {
		return nil, nil
	}
	mi, err := metainfo.Load(r)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load torrent")
	}
	return mi, nil
}
