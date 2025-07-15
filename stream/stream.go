/*
Package stream provides http.Handler middleware that passes-through the entire request

Stream works around several limitations caused by buffering implementations, but
also introduces certain risks.

Workarounds for buffering limitations:
1. Streaming really large chunks of data (large file transfers, or streaming videos,
etc.)

2. Streaming (chunking) sparse data. For example, an implementation might
send a health check or a heart beat over a long-lived connection. This
does not play well with buffering.

Risks:
1. Connections could survive for very long periods of time.

2. There is no easy way to enforce limits on size/time of a connection.

Examples of a streaming middleware:

	// sample HTTP handler.
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	  w.Write([]byte("hello"))
	})

	// Stream will literally pass through to the next handler without ANY buffering
	// or validation of the data.
	stream.New(handler)
*/
package stream

import (
	"net/http"

	"github.com/vulcand/oxy/v2/utils"
)

// DefaultMaxBodyBytes No limit by default.
const DefaultMaxBodyBytes = -1

// Stream is responsible for buffering requests and responses
// It buffers large requests and responses to disk,.
type Stream struct {
	maxRequestBodyBytes int64

	maxResponseBodyBytes int64

	next http.Handler

	verbose bool
	log     utils.Logger
}

// New returns a new streamer middleware. New() function supports optional functional arguments.
func New(next http.Handler, setters ...Option) (*Stream, error) {
	strm := &Stream{
		next: next,

		maxRequestBodyBytes: DefaultMaxBodyBytes,

		maxResponseBodyBytes: DefaultMaxBodyBytes,

		log: &utils.NoopLogger{},
	}
	for _, s := range setters {
		if err := s(strm); err != nil {
			return nil, err
		}
	}

	return strm, nil
}

// Wrap sets the next handler to be called by stream handler.
func (s *Stream) Wrap(next http.Handler) error {
	s.next = next
	return nil
}

func (s *Stream) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if s.verbose {
		dump := utils.DumpHTTPRequest(req)

		s.log.Debug("vulcand/oxy/stream: begin ServeHttp on request: %s", dump)
		defer s.log.Debug("vulcand/oxy/stream: completed ServeHttp on request: %s", dump)
	}

	s.next.ServeHTTP(w, req)
}
