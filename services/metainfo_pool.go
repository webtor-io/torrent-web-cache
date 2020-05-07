package services

import (
	"context"
	"sync"

	"github.com/anacrolix/torrent/metainfo"
)

type MetaInfoPool struct {
	sm sync.Map
	st *S3Storage
}

func NewMetaInfoPool(st *S3Storage) *MetaInfoPool {
	return &MetaInfoPool{st: st}
}

func (s *MetaInfoPool) Get(ctx context.Context, h string) (*metainfo.MetaInfo, error) {
	v, loaded := s.sm.LoadOrStore(h, NewMetaInfoLoader(ctx, h, s.st))
	if !loaded {
		go func() {
			<-ctx.Done()
			s.sm.Delete(h)
		}()
	}

	return v.(*MetaInfoLoader).Get()
}
