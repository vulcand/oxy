package netutils

import (
	"net/http"
)

type ProxyWriter struct {
	W    http.ResponseWriter
	Code int
}

func (p *ProxyWriter) Header() http.Header {
	return p.W.Header()
}

func (p *ProxyWriter) Write(buf []byte) (int, error) {
	return p.W.Write(buf)
}

func (p *ProxyWriter) WriteHeader(code int) {
	p.Code = code
	p.WriteHeader(code)
}
