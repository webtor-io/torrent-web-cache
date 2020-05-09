package services

import (
	"context"
)

type ReaderPool struct {
	pp  *PiecePool
	mip *MetaInfoPool
	ttp *TorrentTouchPool
}

func NewReaderPool(pp *PiecePool, mip *MetaInfoPool, ttp *TorrentTouchPool) *ReaderPool {
	return &ReaderPool{mip: mip, pp: pp, ttp: ttp}
}

func (rp *ReaderPool) Get(ctx context.Context, url string, rate string) (*Reader, error) {
	return NewReader(ctx, rp.mip, rp.pp, rp.ttp, url, rate)
}
