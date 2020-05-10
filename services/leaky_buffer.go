package services

type LeakyBuffer struct {
	c chan []byte
}

func NewLeakyBuffer(size int, bufSize int64) *LeakyBuffer {
	c := make(chan []byte, size)
	for i := 0; i < size; i++ {
		c <- make([]byte, bufSize)
	}
	return &LeakyBuffer{c: c}
}

func (s *LeakyBuffer) Get() []byte {
	return <-s.c
}

func (s *LeakyBuffer) Put(b []byte) {
	s.c <- b
}
