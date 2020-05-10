package services

import (
	"context"
)

type ReaderPool struct {
	pp  *PiecePool
	mip *MetaInfoPool
	ttp *TorrentTouchPool
	lb  *LeakyBuffer
}

func NewReaderPool(pp *PiecePool, mip *MetaInfoPool, ttp *TorrentTouchPool, lb *LeakyBuffer) *ReaderPool {
	return &ReaderPool{mip: mip, pp: pp, ttp: ttp, lb: lb}
}

func (rp *ReaderPool) Get(ctx context.Context, url string, rate string) (*Reader, error) {
	return NewReader(ctx, rp.mip, rp.pp, rp.ttp, rp.lb, url, rate)
}
