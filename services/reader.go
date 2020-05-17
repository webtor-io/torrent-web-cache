package services

import (
	"context"
	"io"
	"net/url"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
	log "github.com/sirupsen/logrus"

	"code.cloudfoundry.org/bytefmt"
	"github.com/juju/ratelimit"
	"github.com/pkg/errors"
)

type Reader struct {
	pp          *PiecePool
	ttp         *TorrentTouchPool
	mip         *MetaInfoPool
	src         string
	query       string
	hash        string
	path        string
	redirectURL string
	touch       bool
	offset      int64
	fiOffset    int64
	fileInfo    *metainfo.FileInfo
	info        *metainfo.Info
	pn          int64
	cr          io.ReadCloser
	ctx         context.Context
	N           int64
	rate        string
	lb          *LeakyBuffer
}

func NewReader(ctx context.Context, mip *MetaInfoPool, pp *PiecePool, ttp *TorrentTouchPool, lb *LeakyBuffer, s string, rate string) (*Reader, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse source url=%v", s)
	}
	parts := strings.SplitN(u.Path, "/", 3)
	hash := parts[1]
	path := parts[2]
	src := u.Scheme + "://" + u.Host
	query := u.RawQuery
	redirectURL := u.RequestURI()
	return &Reader{lb: lb, ttp: ttp, pp: pp, mip: mip, src: src, query: query, hash: hash, path: path, redirectURL: redirectURL, offset: 0, touch: false, ctx: ctx, N: -1, rate: rate}, nil
}

func (r *Reader) Path() string {
	return r.path
}

func (r *Reader) RedirectURL() string {
	return r.redirectURL
}

func (r *Reader) Ready() (bool, error) {
	mi, err := r.mip.Get(r.hash)
	if err != nil {
		return false, errors.Wrap(err, "Failed to get ready state")
	}
	return mi != nil, nil
}

func (r *Reader) getInfo() (*metainfo.Info, error) {
	if r.info != nil {
		return r.info, nil
	}
	mi, err := r.mip.Get(r.hash)
	if err != nil {
		return nil, err
	}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, err
	}
	r.info = &info
	return &info, nil
}
func (r *Reader) getFileInfo() (*metainfo.FileInfo, int64, error) {
	if r.fileInfo != nil {
		return r.fileInfo, r.fiOffset, nil
	}
	info, err := r.getInfo()
	if err != nil {
		return nil, 0, err
	}
	for _, f := range info.UpvertedFiles() {
		tt := []string{}
		tt = append(tt, info.Name)
		tt = append(tt, f.Path...)
		if strings.Join(tt, "/") == r.path {
			r.fileInfo = &f
			r.fiOffset = f.Offset(info)
			return &f, r.fiOffset, nil
		}
	}
	return nil, 0, errors.Errorf("File not found path=%v infohash=%v", r.path, r.hash)
}

func (r *Reader) getReader(limit int64, rate uint64) (io.Reader, error) {
	if !r.touch {
		r.touch = true
		defer func() {
			if err := r.ttp.Touch(r.hash); err != nil {
				log.WithError(err).Error("Failed to touch torrent")
			}
		}()
	}
	fi, fiOffset, err := r.getFileInfo()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get FileInfo")
	}
	if fi.Length < r.offset {
		return nil, io.EOF
	}
	i, err := r.getInfo()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get Info")
	}
	offset := r.offset + fiOffset
	pieceNum := offset / i.PieceLength
	piece := i.Piece(int(pieceNum))
	pieceLength := piece.Length()
	start := piece.Offset()
	pieceStart := offset - start
	pieceEnd := piece.Length() - 1
	if start+piece.Length() > fiOffset+r.offset+limit {
		pieceEnd = fiOffset + +r.offset + limit - start - 1
	}
	full := pieceEnd-pieceStart == pieceLength-1
	var pr io.ReadCloser
	if r.cr == nil {
		pr, err = r.pp.Get(r.ctx, r.src, r.hash, piece.Hash().HexString(), r.query, pieceStart, pieceEnd, full)
	} else if r.cr != nil && pieceNum != r.pn {
		r.cr.Close()
		pr, err = r.pp.Get(r.ctx, r.src, r.hash, piece.Hash().HexString(), r.query, pieceStart, pieceEnd, full)
	} else {
		pr = r.cr
	}
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get Piece data")
	}
	r.cr = pr
	r.pn = pieceNum

	var rrr io.Reader
	if rate != 0 {
		if err != nil {
			return nil, err
		}
		bucket := ratelimit.NewBucketWithRate(float64(rate), int64(rate))
		rrr = ratelimit.Reader(r.cr, bucket)
	} else {
		rrr = r.cr
	}
	return rrr, nil
}

func (r *Reader) WriteTo(w io.Writer) (n int64, err error) {
	n = 0
	var pr io.Reader
	var nn int64

	fi, _, err := r.getFileInfo()

	if err != nil {
		return 0, errors.Wrap(err, "Failed to get FileInfo")
	}
	limit := fi.Length - r.offset
	if r.N != -1 {
		limit = r.N
	}

	var rate uint64 = 0
	if r.rate != "" {
		rate, err = bytefmt.ToBytes(r.rate)
		if err != nil {
			return 0, errors.Wrap(err, "Failed to parse rate")
		}
	}

	for {
		if r.ctx.Err() != nil {
			log.WithError(r.ctx.Err()).Error("Got context error")
			return n, r.ctx.Err()
		}
		pr, err = r.getReader(limit, rate)
		if err != nil {
			return
		}
		buf := r.lb.Get()
		nn, err = io.CopyBuffer(w, pr, buf)
		r.lb.Put(buf)
		n = n + nn

		r.offset = r.offset + nn
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
	panic("Not implemented")
}

func (r *Reader) Close() error {
	if r.cr != nil {
		r.cr.Close()
	}
	return nil
}

func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	fi, _, err := r.getFileInfo()
	if err != nil {
		log.WithError(err).Error("Failed to get FileInfo")
		return 0, errors.Wrap(err, "Failed to get FileInfo")
	}
	newOffset := int64(0)
	switch whence {
	case io.SeekStart:
		newOffset = offset
		break
	case io.SeekCurrent:
		newOffset = r.offset + offset
		break
	case io.SeekEnd:
		newOffset = fi.Length + offset
		break
	}
	r.offset = newOffset
	return newOffset, nil
}
