// package forwarder implements http handler that forwards requests to remote server
// and serves back the response
// websocket proxying support based on https://github.com/yhat/wsutil
package forward

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"crypto/tls"
	"net"
	"net/http/httputil"
	"reflect"

	log "github.com/sirupsen/logrus"
	"github.com/vulcand/oxy/utils"
)

// ReqRewriter can alter request headers and body
type ReqRewriter interface {
	Rewrite(r *http.Request)
}

type optSetter func(f *Forwarder) error

// PassHostHeader specifies if a client's Host header field should
// be delegated
func PassHostHeader(b bool) optSetter {
	return func(f *Forwarder) error {
		f.httpForwarder.passHost = b
		return nil
	}
}

// RoundTripper sets a new http.RoundTripper
// Forwarder will use http.DefaultTransport as a default round tripper
func RoundTripper(r http.RoundTripper) optSetter {
	return func(f *Forwarder) error {
		f.httpForwarder.roundTripper = r
		return nil
	}
}

// Rewriter defines a request rewriter for the HTTP forwarder
func Rewriter(r ReqRewriter) optSetter {
	return func(f *Forwarder) error {
		f.httpForwarder.rewriter = r
		return nil
	}
}

// PassHostHeader specifies if a client's Host header field should
// be delegated
func WebsocketTLSClientConfig(tcc *tls.Config) optSetter {
	return func(f *Forwarder) error {
		f.httpForwarder.tlsClientConfig = tcc
		return nil
	}
}

// ErrorHandler is a functional argument that sets error handler of the server
func ErrorHandler(h utils.ErrorHandler) optSetter {
	return func(f *Forwarder) error {
		f.errHandler = h
		return nil
	}
}

// Stream specifies if HTTP responses should be streamed.
func Stream(stream bool) optSetter {
	return func(f *Forwarder) error {
		f.stream = stream
		return nil
	}
}

// Logger defines the logger the forwarder will use.
//
// It defaults to logrus.StandardLogger(), the global logger used by logrus.
func Logger(l *log.Logger) optSetter {
	return func(f *Forwarder) error {
		f.log = l
		return nil
	}
}

func StateListener(stateListener UrlForwardingStateListener) optSetter {
	return func(f *Forwarder) error {
		f.stateListener = stateListener
		return nil
	}
}

func StreamingFlushInterval(flushInterval time.Duration) optSetter {
	return func(f *Forwarder) error {
		f.httpForwarder.flushInterval = flushInterval
		return nil
	}
}

// Forwarder wraps two traffic forwarding implementations: HTTP and websockets.
// It decides based on the specified request which implementation to use
type Forwarder struct {
	*httpForwarder
	*handlerContext
	stateListener UrlForwardingStateListener
	stream        bool
}

// handlerContext defines a handler context for error reporting and logging
type handlerContext struct {
	errHandler utils.ErrorHandler
}

// httpForwarder is a handler that can reverse proxy
// HTTP traffic
type httpForwarder struct {
	roundTripper  http.RoundTripper
	rewriter      ReqRewriter
	passHost      bool
	flushInterval time.Duration

	tlsClientConfig *tls.Config

	log *log.Logger
}

const (
	StateConnected = iota
	StateDisconnected
)

type UrlForwardingStateListener func(*url.URL, int)

// New creates an instance of Forwarder based on the provided list of configuration options
func New(setters ...optSetter) (*Forwarder, error) {
	f := &Forwarder{
		httpForwarder:  &httpForwarder{log: log.StandardLogger()},
		handlerContext: &handlerContext{},
	}
	for _, s := range setters {
		if err := s(f); err != nil {
			return nil, err
		}
	}

	if !f.stream {
		f.flushInterval = 0
	} else if f.flushInterval == 0 {
		f.flushInterval = time.Duration(100) * time.Millisecond
	}

	if f.httpForwarder.rewriter == nil {
		h, err := os.Hostname()
		if err != nil {
			h = "localhost"
		}
		f.httpForwarder.rewriter = &HeaderRewriter{TrustForwardHeader: true, Hostname: h}
	}

	if f.httpForwarder.roundTripper == nil {
		f.httpForwarder.roundTripper = http.DefaultTransport
	}

	if f.errHandler == nil {
		f.errHandler = utils.DefaultHandler
	}
	return f, nil
}

// ServeHTTP decides which forwarder to use based on the specified
// request and delegates to the proper implementation
func (f *Forwarder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if f.log.Level >= log.DebugLevel {
		logEntry := f.log.WithField("Request", utils.DumpHttpRequest(req))
		logEntry.Debug("vulcand/oxy/forward: begin ServeHttp on request")
		defer logEntry.Debug("vulcand/oxy/forward: competed ServeHttp on request")
	}

	if f.stateListener != nil {
		f.stateListener(req.URL, StateConnected)
		defer f.stateListener(req.URL, StateDisconnected)
	}
	if IsWebsocketRequest(req) {
		f.httpForwarder.serveWebSocket(w, req, f.handlerContext)
	} else {
		f.httpForwarder.serveHTTP(w, req, f.handlerContext)
	}
}

// copyRequest makes a copy of the specified request to be sent using the configured
// transport
func (f *httpForwarder) copyRequest(req *http.Request, u *url.URL) *http.Request {
	outReq := new(http.Request)
	*outReq = *req // includes shallow copies of maps, but we handle this below

	outReq.URL = utils.CopyURL(req.URL)
	outReq.URL.Scheme = u.Scheme
	outReq.URL.Host = u.Host
	outReq.URL.Opaque = req.RequestURI
	// raw query is already included in RequestURI, so ignore it to avoid dupes
	outReq.URL.RawQuery = ""
	// Do not pass client Host header unless optsetter PassHostHeader is set.
	if !f.passHost {
		outReq.Host = u.Host
	}

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1

	if f.rewriter != nil {
		f.rewriter.Rewrite(outReq)
	}
	return outReq
}

// serveHTTP forwards websocket traffic
func (f *httpForwarder) serveWebSocket(w http.ResponseWriter, req *http.Request, ctx *handlerContext) {
	if f.log.Level >= log.DebugLevel {
		logEntry := f.log.WithField("Request", utils.DumpHttpRequest(req))
		logEntry.Debug("vulcand/oxy/forward/websocket: begin ServeHttp on request")
		defer logEntry.Debug("vulcand/oxy/forward/websocket: competed ServeHttp on request")
	}

	outReq := f.copyWebSocketRequest(req)
	host := outReq.URL.Host

	// if host does not specify a port, use the default http port
	if !strings.Contains(host, ":") {
		if outReq.URL.Scheme == "wss" {
			host = host + ":443"
		} else {
			host = host + ":80"
		}
	}

	var targetConn net.Conn
	var err error

	if outReq.URL.Scheme == "wss" && f.tlsClientConfig != nil {
		f.log.Debugf("vulcand/oxy/forward/websocket: Dialing secure (tls) tcp connection to host %s with TLS Client Config %v", host, f.tlsClientConfig)
		targetConn, err = tls.Dial("tcp", host, f.tlsClientConfig)
	} else {
		f.log.Debugf("vulcand/oxy/forward/websocket: Dialing insecure (non-tls) tcp connection to host %s", host)
		targetConn, err = net.Dial("tcp", host)
	}

	if err != nil {
		f.log.Errorf("vulcand/oxy/forward/websocket: Error dialing `%v`: %v", host, err)
		ctx.errHandler.ServeHTTP(w, req, err)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		f.log.Errorf("vulcand/oxy/forward/websocket: Unable to hijack the connection: does not implement http.Hijacker. ResponseWriter implementation type: %v", reflect.TypeOf(w))
		ctx.errHandler.ServeHTTP(w, req, err)
		return
	}
	underlyingConn, _, err := hijacker.Hijack()
	if err != nil {
		f.log.Errorf("vulcand/oxy/forward/websocket: Unable to hijack the connection: %v", err)
		ctx.errHandler.ServeHTTP(w, req, err)
		return
	}
	// it is now caller's responsibility to Close the underlying connection
	defer underlyingConn.Close()
	defer targetConn.Close()

	f.log.Infof("vulcand/oxy/forward/websocket: Writing outgoing Websocket request to target connection: %+v", outReq)

	// write the modified incoming request to the dialed connection
	if err = outReq.Write(targetConn); err != nil {
		f.log.Errorf("vulcand/oxy/forward/websocket: Unable to copy request to target: %v", err)
		ctx.errHandler.ServeHTTP(w, req, err)
		return
	}
	errc := make(chan error, 2)
	replicate := func(dst io.Writer, src io.Reader, dstName string, srcName string) {
		_, err := io.Copy(dst, src)
		if err != nil {
			f.log.Errorf("vulcand/oxy/forward/websocket: Error when copying from %s to %s using io.Copy: %v", srcName, dstName, err)
		} else {
			f.log.Infof("vulcand/oxy/forward/websocket: Copying from %s to %s using io.Copy completed without error.", srcName, dstName)
		}
		errc <- err
	}
	go replicate(targetConn, underlyingConn, "backend", "client")
	go replicate(underlyingConn, targetConn, "client", "backend")
	err = <-errc // One goroutine complete
	f.log.Infof("vulcand/oxy/forward/websocket: first proxying connection closed: %v", err)
	err = <-errc // Both goroutines complete
	f.log.Infof("vulcand/oxy/forward/websocket: second proxying connection closed: %v", err)
}

// copyRequest makes a copy of the specified request.
func (f *httpForwarder) copyWebSocketRequest(req *http.Request) (outReq *http.Request) {
	outReq = new(http.Request)
	*outReq = *req
	outReq.URL = utils.CopyURL(req.URL)

	//a good working default
	outReq.URL.Scheme = req.URL.Scheme

	//sometimes backends might be registered as HTTP/HTTPS servers so translate URLs to websocket URLs.
	switch req.URL.Scheme {
	case "https":
		outReq.URL.Scheme = "wss"
	case "http":
		outReq.URL.Scheme = "ws"
	}

	outReq.URL.Host = req.URL.Host
	outReq.URL.Opaque = req.RequestURI

	// Do not pass client Host header unless optsetter PassHostHeader is set.
	if !f.passHost {
		outReq.Host = req.Host
	}

	// Overwrite close flag so we can keep persistent connection for the backend servers
	outReq.Close = false

	outReq.Header = make(http.Header)
	utils.CopyHeaders(outReq.Header, req.Header)

	if f.rewriter != nil {
		f.rewriter.Rewrite(outReq)
	}

	return outReq
}

// serveHTTP forwards HTTP traffic using the configured transport
func (f *httpForwarder) serveHTTP(w http.ResponseWriter, inReq *http.Request, ctx *handlerContext) {
	if f.log.Level >= log.DebugLevel {
		logEntry := f.log.WithField("Request", utils.DumpHttpRequest(inReq))
		logEntry.Debug("vulcand/oxy/forward/http: begin ServeHttp on request")
		defer logEntry.Debug("vulcand/oxy/forward/http: completed ServeHttp on request")
	}

	pw := &utils.ProxyWriter{
		W: w,
	}
	start := time.Now().UTC()

	revproxy := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req = f.copyRequest(req, inReq.URL)
		},
		Transport:     f.roundTripper,
		FlushInterval: f.flushInterval,
	}
	revproxy.ServeHTTP(pw, inReq)

	if inReq.TLS != nil {
		f.log.Infof("vulcand/oxy/forward/http: Round trip: %v, code: %v, Length: %v, duration: %v tls:version: %x, tls:resume:%t, tls:csuite:%x, tls:server:%v",
			inReq.URL, pw.Code, pw.Length, time.Now().UTC().Sub(start),
			inReq.TLS.Version,
			inReq.TLS.DidResume,
			inReq.TLS.CipherSuite,
			inReq.TLS.ServerName)
	} else {
		f.log.Infof("vulcand/oxy/forward/http: Round trip: %v, code: %v, Length: %v, duration: %v",
			inReq.URL, pw.Code, pw.Length, time.Now().UTC().Sub(start))
	}
}

// isWebsocketRequest determines if the specified HTTP request is a
// websocket handshake request
func IsWebsocketRequest(req *http.Request) bool {
	containsHeader := func(name, value string) bool {
		items := strings.Split(req.Header.Get(name), ",")
		for _, item := range items {
			if value == strings.ToLower(strings.TrimSpace(item)) {
				return true
			}
		}
		return false
	}
	return containsHeader(Connection, "upgrade") && containsHeader(Upgrade, "websocket")
}
