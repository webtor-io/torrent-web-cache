package services

import (
	"context"
	"io"
	"net/url"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
	log "github.com/sirupsen/logrus"

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
}

func NewReader(mip *MetaInfoPool, pp *PiecePool, ttp *TorrentTouchPool, s string, ctx context.Context) (*Reader, error) {
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
	return &Reader{ttp: ttp, pp: pp, mip: mip, src: src, query: query, hash: hash, path: path, redirectURL: redirectURL, offset: 0, touch: false, ctx: ctx}, nil
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

func (r *Reader) Read(p []byte) (n int, err error) {
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
		return 0, errors.Wrap(err, "Failed to get FileInfo")
	}
	if fi.Length < r.offset {
		return 0, io.EOF
	}
	i, err := r.getInfo()
	if err != nil {
		return 0, errors.Wrap(err, "Failed to get Info")
	}
	offset := r.offset + fiOffset
	pieceNum := offset / i.PieceLength
	piece := i.Piece(int(pieceNum))
	lastPiece := false
	if fiOffset+fi.Length <= piece.Offset()+piece.Length() {
		lastPiece = true
	}
	start := piece.Offset()
	length := offset - start
	var pr io.ReadCloser
	if r.cr == nil {
		pr, err = r.pp.Get(r.src, r.hash, piece.Hash().HexString(), r.query, length, piece.Length(), r.ctx)
	} else if r.cr != nil && pieceNum != r.pn {
		r.cr.Close()
		pr, err = r.pp.Get(r.src, r.hash, piece.Hash().HexString(), r.query, length, piece.Length(), r.ctx)
	} else {
		pr = r.cr
	}
	if err != nil {
		return 0, errors.Wrap(err, "Failed to get Piece data")
	}
	r.cr = pr
	r.pn = pieceNum
	// pr.Seek(length, io.SeekStart)
	n, err = pr.Read(p)
	// lr := io.LimitReader(pr, start+length-offset)
	// n, err = lr.Read(p)
	r.offset = r.offset + int64(n)
	if err != nil && err != io.EOF {
		log.WithError(err).Error("Failed to read Piece data")
		return
	} else if err == io.EOF && lastPiece {
		if r.cr != nil {
			r.cr.Close()
		}
		return n, io.EOF
	}
	return n, nil
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
