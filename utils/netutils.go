package utils

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
)

// ProxyWriter calls recorder, used to debug logs.
type ProxyWriter struct {
	w      http.ResponseWriter
	code   int
	length int64

	log Logger
}

// NewProxyWriter creates a new ProxyWriter.
func NewProxyWriter(w http.ResponseWriter) *ProxyWriter {
	return NewProxyWriterWithLogger(w, &NoopLogger{})
}

// NewProxyWriterWithLogger creates a new ProxyWriter.
func NewProxyWriterWithLogger(w http.ResponseWriter, l Logger) *ProxyWriter {
	return &ProxyWriter{
		w:   w,
		log: l,
	}
}

// StatusCode gets status code.
func (p *ProxyWriter) StatusCode() int {
	if p.code == 0 {
		// per contract standard lib will set this to http.StatusOK if not set
		// by user, here we avoid the confusion by mirroring this logic
		return http.StatusOK
	}

	return p.code
}

// GetLength gets content length.
func (p *ProxyWriter) GetLength() int64 {
	return p.length
}

// Header gets response header.
func (p *ProxyWriter) Header() http.Header {
	return p.w.Header()
}

func (p *ProxyWriter) Write(buf []byte) (int, error) {
	p.length += int64(len(buf))
	return p.w.Write(buf)
}

// WriteHeader writes status code.
func (p *ProxyWriter) WriteHeader(code int) {
	p.code = code
	p.w.WriteHeader(code)
}

// Flush flush the writer.
func (p *ProxyWriter) Flush() {
	_ = p.FlushError()
}

// FlushError flushes the writer, see http.ResponseController.Flush.
func (p *ProxyWriter) FlushError() error {
	return http.NewResponseController(p.w).Flush()
}

// Hijack lets the caller take over the connection.
func (p *ProxyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(p.w).Hijack()
	if errors.Is(err, http.ErrNotSupported) {
		p.log.Debug("Upstream ResponseWriter of type %v does not implement http.Hijacker.", reflect.TypeOf(p.w))
	}

	return conn, rw, err
}

// Unwrap returns the wrapped ResponseWriter, used by http.ResponseController.
func (p *ProxyWriter) Unwrap() http.ResponseWriter {
	return p.w
}

// BufferWriter buffer writer.
type BufferWriter struct {
	H    http.Header
	Code int
	W    io.WriteCloser
	log  Logger
}

// NewBufferWriter creates a new BufferWriter.
func NewBufferWriter(w io.WriteCloser, l Logger) *BufferWriter {
	return &BufferWriter{
		W:   w,
		H:   make(http.Header),
		log: l,
	}
}

// Close close the writer.
func (b *BufferWriter) Close() error {
	return b.W.Close()
}

// Header gets response header.
func (b *BufferWriter) Header() http.Header {
	return b.H
}

func (b *BufferWriter) Write(buf []byte) (int, error) {
	return b.W.Write(buf)
}

// WriteHeader writes status code.
func (b *BufferWriter) WriteHeader(code int) {
	b.Code = code
}

// Hijack lets the caller take over the connection.
func (b *BufferWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hi, ok := b.W.(http.Hijacker); ok {
		return hi.Hijack()
	}

	b.log.Debug("Upstream ResponseWriter of type %v does not implement http.Hijacker. Returning dummy channel.", reflect.TypeOf(b.W))

	return nil, nil, fmt.Errorf("the response writer that was wrapped in this proxy, does not implement http.Hijacker. It is of type: %v", reflect.TypeOf(b.W))
}

type nopWriteCloser struct {
	io.Writer
}

func (*nopWriteCloser) Close() error { return nil }

// NopWriteCloser returns a WriteCloser with a no-op Close method wrapping
// the provided Writer w.
func NopWriteCloser(w io.Writer) io.WriteCloser {
	return &nopWriteCloser{Writer: w}
}

// CopyURL provides update safe copy by avoiding shallow copying User field.
func CopyURL(i *url.URL) *url.URL {
	out := *i
	if i.User != nil {
		u := *i.User
		out.User = &u
	}

	return &out
}

// CopyHeaders copies http headers from source to destination, it
// does not override, but adds multiple headers.
func CopyHeaders(dst http.Header, src http.Header) {
	for k, vv := range src {
		dst[k] = append(dst[k], vv...)
	}
}

// HasHeaders determines whether any of the header names is present in the http headers.
func HasHeaders(names []string, headers http.Header) bool {
	for _, h := range names {
		if headers.Get(h) != "" {
			return true
		}
	}

	return false
}

// RemoveHeaders removes the header with the given names from the headers map.
func RemoveHeaders(headers http.Header, names ...string) {
	for _, h := range names {
		headers.Del(h)
	}
}
