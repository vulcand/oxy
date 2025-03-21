/*
Package buffer provides http.Handler middleware that solves several problems when dealing with http requests:

Reads the entire request and response into buffer, optionally buffering it to disk for large requests.
Checks the limits for the requests and responses, rejecting in case if the limit was exceeded.
Changes request content-transfer-encoding from chunked and provides total size to the handlers.

Examples of a buffering middleware:

	// sample HTTP handler.
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	  w.Write([]byte("hello"))
	})

	// Buffer will read the body in buffer before passing the request to the handler
	// calculate total size of the request and transform it from chunked encoding
	// before passing to the server
	buffer.New(handler)

	// This version will buffer up to 2MB in memory and will serialize any extra
	// to a temporary file, if the request size exceeds 10MB it will reject the request
	buffer.New(handler,
	  buffer.MemRequestBodyBytes(2 * 1024 * 1024),
	  buffer.MaxRequestBodyBytes(10 * 1024 * 1024))

	// Will do the same as above, but with responses
	buffer.New(handler,
	  buffer.MemResponseBodyBytes(2 * 1024 * 1024),
	  buffer.MaxResponseBodyBytes(10 * 1024 * 1024))

	// Buffer will replay the request if the handler returns error at least 3 times
	// before returning the response
	buffer.New(handler, buffer.Retry(`IsNetworkError() && Attempts() <= 2`))
*/
package buffer

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"

	"github.com/mailgun/multibuf"
	"github.com/vulcand/oxy/v2/utils"
)

const (
	// DefaultMemBodyBytes Store up to 1MB in RAM.
	DefaultMemBodyBytes = 1048576
	// DefaultMaxBodyBytes No limit by default.
	DefaultMaxBodyBytes = -1
	// DefaultMaxRetryAttempts Maximum retry attempts.
	DefaultMaxRetryAttempts = 10
)

var errHandler utils.ErrorHandler = &SizeErrHandler{}

// Buffer is responsible for buffering requests and responses
// It buffers large requests and responses to disk,.
type Buffer struct {
	maxRequestBodyBytes int64
	memRequestBodyBytes int64

	maxResponseBodyBytes int64
	memResponseBodyBytes int64

	retryPredicate hpredicate

	next       http.Handler
	errHandler utils.ErrorHandler

	verbose bool
	log     utils.Logger
}

// New returns a new buffer middleware. New() function supports optional functional arguments.
func New(next http.Handler, setters ...Option) (*Buffer, error) {
	strm := &Buffer{
		next: next,

		maxRequestBodyBytes: DefaultMaxBodyBytes,
		memRequestBodyBytes: DefaultMemBodyBytes,

		maxResponseBodyBytes: DefaultMaxBodyBytes,
		memResponseBodyBytes: DefaultMemBodyBytes,

		log: &utils.NoopLogger{},
	}

	for _, s := range setters {
		if err := s(strm); err != nil {
			return nil, err
		}
	}
	if strm.errHandler == nil {
		strm.errHandler = errHandler
	}

	return strm, nil
}

// Wrap sets the next handler to be called by buffer handler.
func (b *Buffer) Wrap(next http.Handler) error {
	b.next = next
	return nil
}

func (b *Buffer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if b.verbose {
		dump := utils.DumpHTTPRequest(req)
		b.log.Debug("vulcand/oxy/buffer: begin ServeHttp on request: %s", dump)
		defer b.log.Debug("vulcand/oxy/buffer: completed ServeHttp on request: %s", dump)
	}

	if err := b.checkLimit(req); err != nil {
		b.log.Error("vulcand/oxy/buffer: request body over limit, err: %v", err)
		b.errHandler.ServeHTTP(w, req, err)
		return
	}

	// Read the body while keeping limits in mind. This reader controls the maximum bytes
	// to read into memory and disk. This reader returns an error if the total request size exceeds the
	// predefined MaxSizeBytes. This can occur if we got chunked request, in this case ContentLength would be set to -1
	// and the reader would be unbounded bufio in the http.Server
	body, err := multibuf.New(req.Body, multibuf.MaxBytes(b.maxRequestBodyBytes), multibuf.MemBytes(b.memRequestBodyBytes))
	if err != nil || body == nil {
		if req.Context().Err() != nil {
			b.log.Error("vulcand/oxy/buffer: error when reading request body, err: %v", req.Context().Err())
			b.errHandler.ServeHTTP(w, req, req.Context().Err())
			return
		}

		b.log.Error("vulcand/oxy/buffer: error when reading request body, err: %v", err)
		b.errHandler.ServeHTTP(w, req, err)
		return
	}

	// Set request body to buffered reader that can replay the read and execute Seek
	// Note that we don't change the original request body as it's handled by the http server
	// and we don't want to mess with standard library
	defer func() {
		if body != nil {
			errClose := body.Close()
			if errClose != nil {
				b.log.Error("vulcand/oxy/buffer: failed to close body, err: %v", errClose)
			}
		}
	}()

	// We need to set ContentLength based on known request size. The incoming request may have been
	// set without content length or using chunked TransferEncoding
	totalSize, err := body.Size()
	if err != nil {
		b.log.Error("vulcand/oxy/buffer: failed to get request size, err: %v", err)
		b.errHandler.ServeHTTP(w, req, err)
		return
	}

	if totalSize == 0 {
		body = nil
	}

	outReq := b.copyRequest(req, body, totalSize)

	attempt := 1
	for {
		// We create a special writer that will limit the response size, buffer it to disk if necessary
		writer, err := multibuf.NewWriterOnce(multibuf.MaxBytes(b.maxResponseBodyBytes), multibuf.MemBytes(b.memResponseBodyBytes))
		if err != nil {
			b.log.Error("vulcand/oxy/buffer: failed create response writer, err: %v", err)
			b.errHandler.ServeHTTP(w, req, err)
			return
		}

		// We are mimicking http.ResponseWriter to replace writer with our special writer
		bw := &bufferWriter{
			header:         make(http.Header),
			buffer:         writer,
			responseWriter: w,
			log:            b.log,
		}
		defer bw.Close()

		b.next.ServeHTTP(bw, outReq)
		if bw.hijacked {
			b.log.Debug("vulcand/oxy/buffer: connection was hijacked downstream. Not taking any action in buffer.")
			return
		}

		if bw.writeError != nil {
			b.log.Error("vulcand/oxy/buffer: failed to copy response, err: %v", bw.writeError)
			b.errHandler.ServeHTTP(w, req, bw.writeError)
			return
		}

		var reader multibuf.MultiReader
		if bw.expectBody(outReq) {
			rdr, err := writer.Reader()
			if err != nil {
				b.log.Error("vulcand/oxy/buffer: failed to read response, err: %v", err)
				b.errHandler.ServeHTTP(w, req, err)
				return
			}
			defer rdr.Close()
			reader = rdr
		}

		if (b.retryPredicate == nil || attempt > DefaultMaxRetryAttempts) ||
			!b.retryPredicate(&context{r: req, attempt: attempt, responseCode: bw.code}) {
			utils.CopyHeaders(w.Header(), bw.Header())
			w.WriteHeader(bw.code)
			if reader != nil {
				_, _ = io.Copy(w, reader)
			}
			return
		}

		attempt++
		if body != nil {
			if _, err := body.Seek(0, 0); err != nil {
				b.log.Error("vulcand/oxy/buffer: failed to rewind response body, err: %v", err)
				b.errHandler.ServeHTTP(w, req, err)
				return
			}
		}

		outReq = b.copyRequest(req, body, totalSize)
		b.log.Debug("vulcand/oxy/buffer: retry Request(%v %v) attempt %v", req.Method, req.URL, attempt)
	}
}

func (b *Buffer) copyRequest(req *http.Request, body io.ReadCloser, bodySize int64) *http.Request {
	o := *req
	o.URL = utils.CopyURL(req.URL)
	o.Header = make(http.Header)
	utils.CopyHeaders(o.Header, req.Header)
	o.ContentLength = bodySize
	// remove TransferEncoding that could have been previously set because we have transformed the request from chunked encoding
	o.TransferEncoding = []string{}
	// http.Transport will close the request body on any error, we are controlling the close process ourselves, so we override the closer here
	if body == nil {
		o.Body = io.NopCloser(req.Body)
	} else {
		o.Body = io.NopCloser(body.(io.Reader))
	}
	return &o
}

func (b *Buffer) checkLimit(req *http.Request) error {
	if b.maxRequestBodyBytes <= 0 {
		return nil
	}
	if req.ContentLength > b.maxRequestBodyBytes {
		return &multibuf.MaxSizeReachedError{MaxSize: b.maxRequestBodyBytes}
	}
	return nil
}

type bufferWriter struct {
	header         http.Header
	code           int
	buffer         multibuf.WriterOnce
	responseWriter http.ResponseWriter
	hijacked       bool
	writeError     error
	log            utils.Logger
}

// RFC2616 #4.4.
func (b *bufferWriter) expectBody(r *http.Request) bool {
	if r.Method == http.MethodHead {
		return false
	}
	if (b.code >= 100 && b.code < 200) || b.code == 204 || b.code == 304 {
		return false
	}
	// refer to https://github.com/vulcand/oxy/issues/113
	// if b.header.Get("Content-Length") == "" && b.header.Get("Transfer-Encoding") == "" {
	// 	return false
	// }
	if b.header.Get("Content-Length") == "0" {
		return false
	}
	// Support for gRPC, gRPC Web.
	if grpcStatus := b.header.Get("Grpc-Status"); grpcStatus != "" && grpcStatus != "0" {
		return false
	}
	return true
}

func (b *bufferWriter) Close() error {
	return b.buffer.Close()
}

func (b *bufferWriter) Header() http.Header {
	return b.header
}

func (b *bufferWriter) Write(buf []byte) (int, error) {
	length, err := b.buffer.Write(buf)
	if err != nil {
		// Since go1.11 (https://github.com/golang/go/commit/8f38f28222abccc505b9a1992deecfe3e2cb85de)
		// if the writer returns an error, the reverse proxy panics
		b.log.Error("write: %v", err)
		length = len(buf)
		b.writeError = err
	}
	return length, nil
}

// WriteHeader sets rw.Code.
func (b *bufferWriter) WriteHeader(code int) {
	b.code = code
}

// CloseNotify CloseNotifier interface - this allows downstream connections to be terminated when the client terminates.
func (b *bufferWriter) CloseNotify() <-chan bool {
	if cn, ok := b.responseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	b.log.Warn("Upstream ResponseWriter of type %v does not implement http.CloseNotifier. Returning dummy channel.", reflect.TypeOf(b.responseWriter))
	return make(<-chan bool)
}

// Hijack This allows connections to be hijacked for websockets for instance.
func (b *bufferWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hi, ok := b.responseWriter.(http.Hijacker); ok {
		conn, rw, err := hi.Hijack()
		if err == nil {
			b.hijacked = true
		}
		return conn, rw, err
	}
	b.log.Warn("Upstream ResponseWriter of type %v does not implement http.Hijacker.", reflect.TypeOf(b.responseWriter))
	return nil, nil, fmt.Errorf("the response writer wrapped in this proxy does not implement http.Hijacker. Its type is: %v", reflect.TypeOf(b.responseWriter))
}

// SizeErrHandler Size error handler.
type SizeErrHandler struct{}

func (e *SizeErrHandler) ServeHTTP(w http.ResponseWriter, req *http.Request, err error) {
	//nolint:errorlint // must be changed
	if _, ok := err.(*multibuf.MaxSizeReachedError); ok {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.Write([]byte(http.StatusText(http.StatusRequestEntityTooLarge)))
		return
	}
	utils.DefaultHandler.ServeHTTP(w, req, err)
}
