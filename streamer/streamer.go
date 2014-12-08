// package streamer provides a special http.Handler middleware that solves several problems when dealing with http requests:
//
// * Reads the entire request and response into buffer, optionally buffering it to disk for large requests
// * Checks the limits for the requests and responses, rejecting in case if the limit was exceeded
// * Changes request content-transfer-encoding from chunked and provides total size to the handlers

package streamer

import (
	"fmt"
	"io"
	"net/http"

	"github.com/mailgun/multibuf"
)

const (
	// Store up to 1MB in RAM
	DefaultMemBodyBytes = 1048576
	// No limit by default
	DefaultMaxBodyBytes = -1
)

// Streamer is responsible for streaming requests and responses
// It buffers large reqeuests and responses to disk,
type Streamer struct {
	maxRequestBodyBytes int64
	memRequestBodyBytes int64

	maxResponseBodyBytes int64
	memResponseBodyBytes int64

	next       http.Handler
	errHandler http.Handler
}

type optSetter func(s *Streamer) error

// ErrorHandler is a functional argument that sets error handler of the server
func ErrorHandler(h http.Handler) optSetter {
	return func(s *Streamer) error {
		s.errHandler = h
		return nil
	}
}

func MaxRequestBodyBytes(m int64) optSetter {
	return func(s *Streamer) error {
		if m < 0 {
			return fmt.Errorf("max bytes should be >= 0 got %d", m)
		}
		s.maxRequestBodyBytes = m
		return nil
	}
}

func MemRequestBodyBytes(m int64) optSetter {
	return func(s *Streamer) error {
		if m < 0 {
			return fmt.Errorf("mem bytes should be >= 0 got %d", m)
		}
		s.memRequestBodyBytes = m
		return nil
	}
}

func MaxResponseBodyBytes(m int64) optSetter {
	return func(s *Streamer) error {
		if m < 0 {
			return fmt.Errorf("max bytes should be >= 0 got %d", m)
		}
		s.maxResponseBodyBytes = m
		return nil
	}
}

func MemResponseBodyBytes(m int64) optSetter {
	return func(s *Streamer) error {
		if m < 0 {
			return fmt.Errorf("mem bytes should be >= 0 got %d", m)
		}
		s.memResponseBodyBytes = m
		return nil
	}
}

func New(next http.Handler, setters ...optSetter) (*Streamer, error) {
	strm := &Streamer{
		next: next,

		maxRequestBodyBytes: DefaultMaxBodyBytes,
		memRequestBodyBytes: DefaultMemBodyBytes,

		maxResponseBodyBytes: DefaultMaxBodyBytes,
		memResponseBodyBytes: DefaultMemBodyBytes,
	}
	for _, s := range setters {
		if err := s(strm); err != nil {
			return nil, err
		}
	}

	return strm, nil
}

func (s *Streamer) Wrap(next http.Handler) error {
	if s.next != nil {
		return fmt.Errorf("this streamer is already wrapping %T", s.next)
	}
	s.next = next
	return nil
}

func (s *Streamer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if s.isOverLimit(req) {
		s.errHandler.ServeHTTP(w, req)
		return
	}

	// Read the body while keeping limits in mind. This reader controls the maximum bytes
	// to read into memory and disk. This reader returns an error if the total request size exceeds the
	// prefefined MaxSizeBytes. This can occur if we got chunked request, in this case ContentLength would be set to -1
	// and the reader would be unbounded bufio in the http.Server
	body, err := multibuf.New(req.Body, multibuf.MaxBytes(s.maxRequestBodyBytes), multibuf.MemBytes(s.memRequestBodyBytes))
	if err != nil || body == nil {
		s.errHandler.ServeHTTP(w, req)
		return
	}

	// Set request body to buffered reader that can replay the read and execute Seek
	// Note that we don't change the original request body as it's handled by the http server
	// and we don'w want to mess with standard library
	defer body.Close()

	outreq := *req
	outreq.Body = body

	// We need to set ContentLength based on known request size. The incoming request may have been
	// set without content length or using chunked TransferEncoding
	totalSize, err := body.Size()
	if err != nil {
		s.errHandler.ServeHTTP(w, req)
		return
	}
	req.ContentLength = totalSize
	// remove TransferEncoding that could have been previously set because we have transformed the request from chunked encoding
	req.TransferEncoding = []string{}

	// We create a special writer that will limit the response size, buffer it to disk if necessary
	writer, err := multibuf.NewWriterOnce(multibuf.MaxBytes(s.maxResponseBodyBytes), multibuf.MemBytes(s.memResponseBodyBytes))
	if err != nil {
		s.errHandler.ServeHTTP(w, req)
		return
	}

	// We are mimicking http.ResponseWriter to replace writer with our special writer
	b := &bufferWriter{
		header: make(http.Header),
		buffer: writer,
	}
	defer b.Close()

	s.next.ServeHTTP(b, &outreq)

	reader, err := writer.Reader()
	if err != nil {
		s.errHandler.ServeHTTP(w, req)
		return
	}
	defer reader.Close()

	copyHeaders(w.Header(), b.Header())

	w.WriteHeader(b.code)
	io.Copy(w, reader)
}

func (s *Streamer) isOverLimit(req *http.Request) bool {
	if s.maxRequestBodyBytes <= 0 {
		return false
	}
	return req.ContentLength > s.maxRequestBodyBytes
}

type bufferWriter struct {
	header http.Header
	code   int
	buffer multibuf.WriterOnce
}

func (b *bufferWriter) Close() error {
	return b.buffer.Close()
}

func (b *bufferWriter) Header() http.Header {
	return b.header
}

func (b *bufferWriter) Write(buf []byte) (int, error) {
	return b.buffer.Write(buf)
}

// WriteHeader sets rw.Code.
func (b *bufferWriter) WriteHeader(code int) {
	b.code = code
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
