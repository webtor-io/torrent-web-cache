package services

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

type ReaderPool struct {
	pp  *PiecePool
	mip *MetaInfoPool
	ttp *TorrentTouchPool
	lb  *LeakyBuffer
	ppp *PreloadPiecePool
	pqp *PreloadQueuePool
}

func NewReaderPool(pp *PiecePool, mip *MetaInfoPool, ttp *TorrentTouchPool, lb *LeakyBuffer, ppp *PreloadPiecePool, pqp *PreloadQueuePool) *ReaderPool {
	return &ReaderPool{mip: mip, pp: pp, ttp: ttp, lb: lb, ppp: ppp, pqp: pqp}
}

func (rp *ReaderPool) Get(ctx context.Context, s string, piece string, pid string) (*Reader, *url.URL, string, string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, nil, "", "", errors.Wrapf(err, "Failed to parse source url=%v", s)
	}
	parts := strings.SplitN(u.Path, "/", 3)
	hash := parts[1]
	path := parts[2]
	src := u.Scheme + "://" + u.Host
	query := u.RawQuery
	info, err := rp.mip.Get(hash)
	if err != nil {
		return nil, nil, "", "", errors.Wrap(err, "Failed to get MetaInfo")
	}
	if info == nil {
		return nil, u, "", "", nil
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
			return nil, nil, "", "", errors.Errorf("Failed to find piece=%v", piece)
		}
		path = "/" + piece
	} else {
		found := false
		for _, f := range info.UpvertedFiles() {
			tt := []string{}
			tt = append(tt, info.Name)
			tt = append(tt, f.Path...)
			if strings.Join(tt, "/") == path {
				offset = f.Offset(info)
				length = f.Length
				found = true
			}
		}

		if !found {
			return nil, nil, "", "", errors.Errorf("File not found path=%v infohash=%v", path, hash)
		}
	}
	tr := NewReader(ctx, rp.mip, rp.ppp, rp.ttp, rp.lb, rp.pqp, src, hash, query, offset, length, pid)
	if ok, err := tr.Ready(); err != nil {
		return nil, nil, "", "", errors.Wrap(err, "Failed to get reader ready state")
	} else if !ok {
		return nil, nil, "", "", nil
	}
	return tr, nil, path, fmt.Sprintf("%x", sha1.Sum([]byte(hash+path))), nil
}
