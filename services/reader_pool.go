package services

type ReaderPool struct {
	cpp    *CompletedPiecesPool
	mip    *MetaInfoPool
	s3pp   *S3PiecePool
	httppp *HTTPPiecePool
	ttp    *TorrentTouchPool
}

func NewReaderPool(cpp *CompletedPiecesPool, mip *MetaInfoPool, s3pp *S3PiecePool, httppp *HTTPPiecePool, ttp *TorrentTouchPool) *ReaderPool {
	return &ReaderPool{cpp: cpp, mip: mip, s3pp: s3pp, httppp: httppp, ttp: ttp}
}

func (rp *ReaderPool) Get(url string) (*Reader, error) {
	return NewReader(rp.cpp, rp.mip, rp.s3pp, rp.httppp, rp.ttp, url)
}
