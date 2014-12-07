package streamer

import (
	"fmt"
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
	maxBodyBytes int64
	memBodyBytes int64

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

func MaxBodyBytes(m int64) optSetter {
	return func(s *Streamer) error {
		if m < 0 {
			return fmt.Errorf("max bytes should be >= 0 got %d", m)
		}
		s.maxBodyBytes = m
		return nil
	}
}

func MemBodyBytes(m int64) optSetter {
	return func(s *Streamer) error {
		if m < 0 {
			return fmt.Errorf("mem bytes should be >= 0 got %d", m)
		}
		s.memBodyBytes = m
		return nil
	}
}

func New(next http.Handler, setters ...optSetter) (*Streamer, error) {
	strm := &Streamer{
		next:         next,
		maxBodyBytes: DefaultMaxBodyBytes,
		memBodyBytes: DefaultMemBodyBytes,
	}
	for _, s := range setters {
		if err := s(strm); err != nil {
			return nil, err
		}
	}

	return strm, nil
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
	body, err := multibuf.New(req.Body, multibuf.MaxBytes(s.maxBodyBytes), multibuf.MemBytes(s.memBodyBytes))
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

	b := &bufferWriter{
		header: make(http.Header),
		buffer: multibuf.NewWriter(multibuf.MaxBytes(s.maxBodyBytes), multibuf.MemBytes(s.memBodyBytes)),
	}
	defer b.buffer.Close()

	s.next.ServeHTTP(b, &outreq)

	if proxyError.Headers() != nil {
		netutils.CopyHeaders(w.Header(), proxyError.Headers())
	}
	w.WriteHeader(statusCode)
	w.Write(body)

}

func (s *Streamer) isOverLimit(req *http.Request) bool {
	if s.maxBodyBytes <= 0 {
		return false
	}
	return req.ContentLength > s.maxBodyBytes
}

type bufferWriter struct {
	header http.Header
	code   int
	buffer multibuf.WriterOnce
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
