package services

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/webtor-io/lazymap"
)

type HTTPProxyMap struct {
	lazymap.LazyMap
}

func NewHTTPProxyMap() *HTTPProxyMap {
	return &HTTPProxyMap{
		LazyMap: lazymap.New(&lazymap.Config{}),
	}
}
func (s *HTTPProxyMap) get(u *url.URL) *httputil.ReverseProxy {
	uu, _ := url.Parse(u.Scheme + "://" + u.Host)
	pr := httputil.NewSingleHostReverseProxy(uu)
	pr.Director = func(r *http.Request) {
		r.Header["X-Forwarded-For"] = nil
	}
	return pr
}

func (s *HTTPProxyMap) Get(u *url.URL) *httputil.ReverseProxy {
	v, _ := s.LazyMap.Get(u.Host, func() (interface{}, error) {
		return s.get(u), nil
	})
	return v.(*httputil.ReverseProxy)
}
