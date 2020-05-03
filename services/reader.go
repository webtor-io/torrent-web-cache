package services

import (
	"bytes"
	"io"
	"net/url"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"
)

type Reader struct {
	s3pp        *S3PiecePool
	httppp      *HTTPPiecePool
	cpp         *CompletedPiecesPool
	ttp         *TorrentTouchPool
	mip         *MetaInfoPool
	src         string
	hash        string
	path        string
	redirectURL string
	offset      int64
	fileInfo    *metainfo.FileInfo
	info        *metainfo.Info
}

func NewReader(cpp *CompletedPiecesPool, mip *MetaInfoPool, s3pp *S3PiecePool, httppp *HTTPPiecePool, ttp *TorrentTouchPool, s string) (*Reader, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse source url=%v", s)
	}
	parts := strings.SplitN(u.Path, "/", 3)
	hash := parts[1]
	path := parts[2]
	src := u.Scheme + "://" + u.Host
	redirectURL := u.RequestURI()
	return &Reader{ttp: ttp, s3pp: s3pp, httppp: httppp, cpp: cpp, mip: mip, src: src, hash: hash, path: path, redirectURL: redirectURL, offset: 0}, nil
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
func (r *Reader) getFileInfo() (*metainfo.FileInfo, error) {
	if r.fileInfo != nil {
		return r.fileInfo, nil
	}
	info, err := r.getInfo()
	if err != nil {
		return nil, err
	}
	for _, f := range info.UpvertedFiles() {
		if info.Name+"/"+strings.Join(f.Path, "/") == r.path {
			r.fileInfo = &f
			return &f, nil
		}
	}
	return nil, errors.Errorf("File not found path=%v", r.path)
}

func (r *Reader) getPiece(p *metainfo.Piece, i *metainfo.Info, fi *metainfo.FileInfo) (b []byte, start int64, length int64, err error) {
	cp, err := r.cpp.Get(r.hash)
	if err != nil {
		return nil, 0, 0, errors.Wrap(err, "Failed to get Completed Pieces")
	}
	start = p.Offset()
	length = p.Length()
	if cp.Has(p.Hash()) {
		b, err = r.s3pp.Get(r.hash, p.Hash().HexString())
		if b == nil {
			b, err = r.httppp.Get(r.src, r.hash, p.Hash().HexString())
		}
	} else {
		b, err = r.httppp.Get(r.src, r.hash, p.Hash().HexString())
	}
	if terr := r.ttp.Touch(r.hash); terr != nil {
		log.WithError(terr).Error("Failed to touch torrent")
	}
	return
}

func (r *Reader) Read(p []byte) (n int, err error) {
	fi, err := r.getFileInfo()
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
	offset := r.offset + fi.Offset(i)
	pieceNum := offset / i.PieceLength
	piece := i.Piece(int(pieceNum))
	lastPiece := false
	if fi.Offset(i)+fi.Length <= piece.Offset()+piece.Length() {
		lastPiece = true
	}
	pd, start, length, err := r.getPiece(&piece, i, fi)
	if err != nil {
		return 0, errors.Wrap(err, "Failed to get Piece data")
	}
	pr := bytes.NewReader(pd)
	pr.Seek(offset-start, io.SeekStart)
	lr := io.LimitReader(pr, start+length-offset)
	n, err = lr.Read(p)
	if err != nil {
		log.WithError(err).Error("Failed to read Piece data")
		return 0, errors.Wrap(err, "Failed to read Piece data")
	}
	r.offset = r.offset + int64(n)
	if err != io.EOF {
		return
	} else if err == io.EOF && lastPiece {
		return n, io.EOF
	} else {
		return n, nil
	}
}

func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	fi, err := r.getFileInfo()
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
