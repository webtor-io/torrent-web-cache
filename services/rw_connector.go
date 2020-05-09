package services

import (
	"io"
	"net/http"
)

type RWConnector struct {
	w http.ResponseWriter
}

func NewRWConnector(w http.ResponseWriter) *RWConnector {
	return &RWConnector{w: w}
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
		return io.Copy(s.w, r)
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
