package services

import (
	"encoding/hex"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
)

func split(buf []byte, lim int) [][]byte {
	var chunk []byte
	chunks := make([][]byte, 0, len(buf)/lim+1)
	for len(buf) >= lim {
		chunk, buf = buf[:lim], buf[lim:]
		chunks = append(chunks, chunk)
	}
	if len(buf) > 0 {
		chunks = append(chunks, buf[:len(buf)])
	}
	return chunks
}

type CompletedPieces map[[20]byte]bool

func (cp CompletedPieces) Has(h [20]byte) bool {
	_, ok := map[[20]byte]bool(cp)[h]
	return ok
}

func (cp CompletedPieces) HasHex(h string) (bool, error) {
	a, err := hex.DecodeString(h)
	if err != nil {
		return false, errors.Wrapf(err, "Failed to decode hex hash=%v", h)
	}
	var aa [20]byte
	copy(aa[:20], a)
	_, ok := map[[20]byte]bool(cp)[aa]
	return ok, nil
}

func (cp CompletedPieces) Add(h [20]byte) {
	map[[20]byte]bool(cp)[h] = true
}

func (cp CompletedPieces) FromBytes(data []byte) {
	for _, p := range split(data, 20) {
		var k [20]byte
		copy(k[:], p)
		cp[k] = true
	}
}

func (cp CompletedPieces) Load(r io.Reader) error {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	for _, p := range split(data, 20) {
		var k [20]byte
		copy(k[:], p)
		cp[k] = true
	}
	return nil
}

func (cp CompletedPieces) ToBytes() []byte {
	res := []byte{}
	for k := range cp {
		res = append(res, k[:]...)
	}
	return res
}
