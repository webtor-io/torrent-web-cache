package services

import (
	"bufio"
	"io"

	"github.com/pkg/errors"
)

type ReaderWrapper struct {
	curOffset int64
	offset    int64
	length    int64
	r         io.ReadCloser
	br        *bufio.Reader
}

func NewReaderWrapper(r io.ReadCloser, l int64) *ReaderWrapper {
	return &ReaderWrapper{r: r, br: bufio.NewReader(r), curOffset: 0, offset: 0, length: l}
}

func (r *ReaderWrapper) Read(p []byte) (n int, err error) {
	if r.curOffset < r.offset {
		l := r.offset - r.curOffset
		r.br.Discard(int(l))
	}
	n, err = r.br.Read(p)
	r.offset = r.offset + int64(n)
	r.curOffset = r.offset
	return
}

func (r *ReaderWrapper) Close() error {
	return r.r.Close()
}

func (r *ReaderWrapper) Seek(offset int64, whence int) (int64, error) {
	newOffset := int64(0)
	switch whence {
	case io.SeekStart:
		newOffset = offset
		break
	case io.SeekCurrent:
		newOffset = r.offset + offset
		break
	case io.SeekEnd:
		newOffset = r.length + offset
		break
	}
	if newOffset < r.offset {
		return 0, errors.New("Failed to seek back")
	}
	// l := newOffset - r.offset
	// switch r := r.r.(type) {
	// case io.Seeker:
	// 	r.Seek(l, whence)
	// default:
	// 	io.CopyN(ioutil.Discard, r, l)
	// }
	// _, err := io.CopyN(ioutil.Discard, r.r, newOffset-r.offset)
	// log.Infof("%v %v %v %v %v", r.offset, offset, newOffset, newOffset-r.offset, n)
	// if err != nil {
	// 	return 0, errors.Wrap(err, "Failed to seek")
	// }
	r.offset = newOffset
	return r.offset, nil
}
