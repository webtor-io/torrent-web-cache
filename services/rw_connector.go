package services

import (
	"io"
	"net/http"
)

type RWConnector struct {
	w  http.ResponseWriter
	lb *LeakyBuffer
}

func NewRWConnector(w http.ResponseWriter, lb *LeakyBuffer) *RWConnector {
	return &RWConnector{w: w, lb: lb}
}

func (s *RWConnector) Flush() {
	if w, ok := s.w.(http.Flusher); ok {
		w.Flush()
	}
}

func (s *RWConnector) ReadFrom(r io.Reader) (n int64, err error) {
	if l, ok := r.(*io.LimitedReader); ok {
		if rr, ok := l.R.(*Reader); ok {
			rr.N = l.N
			return rr.WriteTo(s.w)
		}
	} else {
		buf := s.lb.Get()
		n, err = io.CopyBuffer(s.w, r, buf)
		s.lb.Put(buf)
		return
	}
	return
}
func (s *RWConnector) Write(p []byte) (n int, err error) {
	return s.w.Write(p)
}
func (s *RWConnector) WriteHeader(statusCode int) {
	s.w.WriteHeader(statusCode)
}

func (s *RWConnector) Header() http.Header {
	return s.w.Header()
}

func (s *RWConnector) CloseNotify() <-chan bool {
	if w, ok := s.w.(http.CloseNotifier); ok {
		return w.CloseNotify()
	}
	panic("Not implemented")
}
