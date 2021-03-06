package services

import (
	"context"
	"io"

	"github.com/anacrolix/torrent/metainfo"
	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"
)

const (
	MAX_PRELOAD_BYTES = 30_000_000
)

type Reader struct {
	pp          *PreloadPiecePool
	ttp         *TorrentTouchPool
	mip         *MetaInfoPool
	src         string
	query       string
	hash        string
	path        string
	redirectURL string
	touch       bool
	readOffset  int64
	offset      int64
	length      int64
	info        *metainfo.Info
	pn          int64
	cr          io.ReadCloser
	ctx         context.Context
	N           int64
	lb          *LeakyBuffer
	pqp         *PreloadQueuePool
	pid         string
}

func NewReader(ctx context.Context, mip *MetaInfoPool, pp *PreloadPiecePool, ttp *TorrentTouchPool, lb *LeakyBuffer, pqp *PreloadQueuePool, src string, hash string, query string, offset int64, length int64, pid string) *Reader {
	return &Reader{lb: lb, ttp: ttp, pp: pp, mip: mip, pqp: pqp, src: src, query: query,
		hash: hash, readOffset: 0, touch: false, ctx: ctx, N: -1, offset: offset,
		length: length, pid: pid}
}

func (r *Reader) Ready() (bool, error) {
	mi, err := r.mip.Get(r.hash)
	if err != nil {
		return false, errors.Wrap(err, "Failed to get ready state")
	}
	return mi != nil, nil
}

func (r *Reader) getInfo() (*metainfo.Info, error) {
	return r.mip.Get(r.hash)
}

func (r *Reader) getReader(limit int64) (io.Reader, error) {
	if !r.touch {
		r.touch = true
		defer func() {
			go func() {
				if err := r.ttp.Touch(r.hash); err != nil {
					log.WithError(err).Error("Failed to touch torrent")
				}
			}()
		}()
	}
	if r.length < r.readOffset {
		return nil, io.EOF
	}
	i, err := r.getInfo()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get Info")
	}
	offset := r.readOffset + r.offset
	pieceNum := offset / i.PieceLength
	piece := i.Piece(int(pieceNum))
	pieceLength := piece.Length()
	start := piece.Offset()
	pieceStart := offset - start
	pieceEnd := piece.Length() - 1
	if start+piece.Length() > r.offset+r.readOffset+limit {
		pieceEnd = r.offset + r.readOffset + limit - start - 1
	}
	full := pieceEnd-pieceStart == pieceLength-1
	// Preload
	preloadBytes := int64(float64(r.length) * 0.05)
	if preloadBytes > MAX_PRELOAD_BYTES {
		preloadBytes = MAX_PRELOAD_BYTES
	}
	preloadSize := preloadBytes / pieceLength
	if r.pn != pieceNum {
		for ii := pieceNum + 1; ii < pieceNum+preloadSize+1 && ii < int64(i.NumPieces()); ii++ {
			r.pqp.Push(r.pid, r.src, r.hash, i.Piece(int(ii)).Hash().HexString(), r.query)
		}
	}
	var pr io.ReadCloser
	if r.cr == nil {
		pr, err = r.pp.Get(r.ctx, r.src, r.hash, piece.Hash().HexString(), r.query, pieceStart, pieceEnd, full)
	} else if r.cr != nil && pieceNum != r.pn {
		r.cr.Close()
		pr, err = r.pp.Get(r.ctx, r.src, r.hash, piece.Hash().HexString(), r.query, pieceStart, pieceEnd, full)
	} else {
		pr = r.cr
	}
	r.pqp.Push(r.pid, r.src, r.hash, i.Piece(int(pieceNum)).Hash().HexString(), r.query)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get Piece data")
	}
	r.cr = pr
	r.pn = pieceNum

	return r.cr, nil
}

func (r *Reader) WriteTo(w io.Writer) (n int64, err error) {
	n = 0
	var pr io.Reader
	var nn int64

	limit := r.length - r.readOffset
	if r.N != -1 {
		limit = r.N
	}

	for {
		if r.ctx.Err() != nil {
			log.WithError(r.ctx.Err()).Error("Got context error")
			return n, r.ctx.Err()
		}
		pr, err = r.getReader(limit)
		if err != nil {
			return
		}
		buf := r.lb.Get()
		nn, err = io.CopyBuffer(w, pr, buf)
		r.lb.Put(buf)
		n = n + nn

		r.readOffset = r.readOffset + nn
		limit = limit - nn
		if err != nil {
			log.WithError(err).Error("Failed to read Piece data")
			return
		} else if limit <= 0 || nn == 0 {
			if r.cr != nil {
				r.cr.Close()
			}
			return n, io.EOF
		}
	}
}

func (r *Reader) Read(p []byte) (n int, err error) {
	rr, err := r.getReader(r.length - r.readOffset)
	if err != nil {
		return
	}
	n, err = rr.Read(p)
	if err != nil {
		log.WithError(err).Errorf("Failed to read")
	}
	r.readOffset = r.readOffset + int64(n)
	return
}

func (r *Reader) Close() error {
	if r.cr != nil {
		r.cr.Close()
	}
	return nil
}

func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	newOffset := int64(0)
	switch whence {
	case io.SeekStart:
		newOffset = offset
		break
	case io.SeekCurrent:
		newOffset = r.readOffset + offset
		break
	case io.SeekEnd:
		newOffset = r.length + offset
		break
	}
	if newOffset < r.readOffset && r.cr != nil {
		r.cr.Close()
		r.cr = nil
	}
	r.readOffset = newOffset
	return newOffset, nil
}
