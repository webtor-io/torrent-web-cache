package services

import (
	"context"
	"net/url"
	"strings"

	"code.cloudfoundry.org/bytefmt"
	"github.com/pkg/errors"
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

func (rp *ReaderPool) Get(ctx context.Context, s string, rate string, piece string) (*Reader, string, string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, "", "", errors.Wrapf(err, "Failed to parse source url=%v", s)
	}
	var r uint64 = 0
	if rate != "" {
		r, err = bytefmt.ToBytes(rate)
		if err != nil {
			return nil, "", "", errors.Wrapf(err, "Failed to parse rate=%v", rate)
		}
	}
	parts := strings.SplitN(u.Path, "/", 3)
	hash := parts[1]
	path := parts[2]
	src := u.Scheme + "://" + u.Host
	query := u.RawQuery
	mi, err := rp.mip.Get(hash)
	if err != nil {
		return nil, "", "", errors.Wrap(err, "Failed to get MetaInfo")
	}
	if mi == nil {
		return nil, u.RequestURI(), "", nil
	}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, "", "", err
	}

	var offset int64 = 0
	var length int64 = 0

	if piece != "" {
		found := false
		for i := 0; i < info.NumPieces(); i++ {
			p := info.Piece(i)
			if p.Hash().HexString() == piece {
				offset = p.Offset()
				length = p.Length()
				found = true
				break
			}
		}
		if !found {
			return nil, "", "", errors.Errorf("Failed to find piece=%v", piece)
		}
		path = "/" + piece
	} else {
		found := false
		for _, f := range info.UpvertedFiles() {
			tt := []string{}
			tt = append(tt, info.Name)
			tt = append(tt, f.Path...)
			if strings.Join(tt, "/") == path {
				offset = f.Offset(&info)
				length = f.Length
				found = true
			}
		}

		if !found {
			return nil, "", "", errors.Errorf("File not found path=%v infohash=%v", path, hash)
		}
	}
	ppp := NewPreloadPiecePool(rp.pp)
	tr := NewReader(ctx, rp.mip, ppp, rp.ttp, rp.lb, src, hash, query, r, offset, length)
	if ok, err := tr.Ready(); err != nil {
		return nil, "", "", errors.Wrap(err, "Failed to get reader ready state")
	} else if !ok {
		return nil, u.RequestURI(), "", nil
	}
	return tr, "", path, nil
}
