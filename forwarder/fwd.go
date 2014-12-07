// package forwarder implements http handler that forwards requests to remote server
// and serves back the response
package forwarder

import (
	"io"
	"net/http"
	"net/url"
	"os"
)

// Router tells forwarder where to route the request
type Router interface {
	Route(r *http.Request) (*url.URL, error)
}

// ReqRewriter can alter request headers and body
type ReqRewriter interface {
	Rewrite(r *http.Request)
}

type optSetter func(f *Forwarder) error

func RoundTripper(r http.RoundTripper) optSetter {
	return func(f *Forwarder) error {
		f.roundTripper = r
		return nil
	}
}

func Rewriter(r ReqRewriter) optSetter {
	return func(f *Forwarder) error {
		f.rewriter = r
		return nil
	}
}

// ErrorHandler is a functional argument that sets error handler of the server
func ErrorHandler(h http.Handler) optSetter {
	return func(f *Forwarder) error {
		f.errHandler = h
		return nil
	}
}

type Forwarder struct {
	errHandler   http.Handler
	router       Router
	roundTripper http.RoundTripper
	rewriter     ReqRewriter
}

func New(router Router, setters ...optSetter) (*Forwarder, error) {
	f := &Forwarder{
		router: router,
	}
	for _, s := range setters {
		if err := s(f); err != nil {
			return nil, err
		}
	}
	if f.roundTripper == nil {
		f.roundTripper = http.DefaultTransport
	}
	if f.rewriter == nil {
		h, err := os.Hostname()
		if err != nil {
			h = "localhost"
		}
		f.rewriter = &HeaderRewriter{TrustForwardHeader: true, Hostname: h}
	}
	return f, nil
}

func (l *Forwarder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	u, err := l.router.Route(req)
	if err != nil {
		l.errHandler.ServeHTTP(w, req)
		return
	}

	response, err := l.roundTripper.RoundTrip(l.copyRequest(req, u))
	if err != nil {
		l.errHandler.ServeHTTP(w, req)
		return
	}

	copyHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	io.Copy(w, response.Body)
	response.Body.Close()
}

func (l *Forwarder) copyRequest(req *http.Request, u *url.URL) *http.Request {
	outReq := new(http.Request)
	*outReq = *req // includes shallow copies of maps, but we handle this below

	outReq.URL.Scheme = u.Scheme
	outReq.URL.Host = u.Host
	outReq.URL.Opaque = req.RequestURI
	// raw query is already included in RequestURI, so ignore it to avoid dupes
	outReq.URL.RawQuery = ""

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1

	// Overwrite close flag so we can keep persistent connection for the backend servers
	outReq.Close = false

	outReq.Header = make(http.Header)
	copyHeaders(outReq.Header, req.Header)
	return outReq
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
