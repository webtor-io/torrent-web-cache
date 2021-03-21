package services

import (
	"context"
	"sync"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/pkg/errors"
)

type MetaInfoLoader struct {
	st       *S3Storage
	infoHash string
	mux      sync.Mutex
	mi       *metainfo.Info
	err      error
	inited   bool
	ctx      context.Context
}

func NewMetaInfoLoader(ctx context.Context, infoHash string, st *S3Storage) *MetaInfoLoader {
	return &MetaInfoLoader{ctx: ctx, st: st, infoHash: infoHash, inited: false}
}

func (s *MetaInfoLoader) Get() (*metainfo.Info, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.mi, s.err
	}
	s.mi, s.err = s.get()
	s.inited = true
	return s.mi, s.err
}

func (s *MetaInfoLoader) get() (*metainfo.Info, error) {
	r, err := s.st.GetTorrent(s.ctx, s.infoHash)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch torrent")
	}
	if r == nil {
		return nil, nil
	}
	defer r.Close()
	mi, err := metainfo.Load(r)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load torrent")
	}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, err
	}
	return &info, nil
}
