package services

type ReaderPool struct {
	pp  *PiecePool
	mip *MetaInfoPool
	ttp *TorrentTouchPool
}

func NewReaderPool(pp *PiecePool, mip *MetaInfoPool, ttp *TorrentTouchPool) *ReaderPool {
	return &ReaderPool{mip: mip, pp: pp, ttp: ttp}
}

func (rp *ReaderPool) Get(url string) (*Reader, error) {
	return NewReader(rp.mip, rp.pp, rp.ttp, url)
}
