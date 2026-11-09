package buffer

import (
	"bufio"
	"bytes"
	gocontext "context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/forward"
	"github.com/vulcand/oxy/v2/testutils"
	"github.com/vulcand/oxy/v2/utils"
)

func TestBuffer_simple(t *testing.T) {
	srv := testutils.NewServerWithResponse(t, "hello")

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))
}

func TestBuffer_defaultResponseStatus(t *testing.T) {
	for _, test := range []struct {
		name    string
		method  string
		body    string
		headers http.Header
	}{
		{name: "body", method: http.MethodGet, body: "hello"},
		{name: "empty", method: http.MethodGet},
		{name: "headers_only", method: http.MethodGet, headers: http.Header{"Content-Type": {"text/plain"}}},
		{name: "zero_content_length", method: http.MethodGet, headers: http.Header{"Content-Length": {"0"}}},
		{name: "head_empty", method: http.MethodHead},
		{name: "head_body", method: http.MethodHead, body: "hello"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				utils.CopyHeaders(w.Header(), test.headers)

				if test.body != "" {
					_, _ = w.Write([]byte(test.body))
				}
			})
			st, err := New(handler)
			require.NoError(t, err)

			response := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, "http://example.com/", nil)

			require.NotPanics(t, func() { st.ServeHTTP(response, request) })

			assert.Equal(t, http.StatusOK, response.Code)

			if test.method == http.MethodHead {
				assert.Empty(t, response.Body.String())
			} else {
				assert.Equal(t, test.body, response.Body.String())
			}

			for name, values := range test.headers {
				assert.Equal(t, values, response.Header().Values(name))
			}
		})
	}
}

func TestBuffer_defaultResponseStatusRetry(t *testing.T) {
	for _, test := range []struct {
		name     string
		retry    string
		attempts int
	}{
		{name: "retry_success", retry: "ResponseCode() == 200 && Attempts() < 2", attempts: 2},
		{name: "keep_success", retry: "ResponseCode() != 200 && Attempts() < 2", attempts: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			attempts := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts++
				_, _ = fmt.Fprintf(w, "attempt %d", attempts)
			})
			st, err := New(handler, Retry(test.retry))
			require.NoError(t, err)

			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)

			require.NotPanics(t, func() { st.ServeHTTP(response, request) })

			assert.Equal(t, test.attempts, attempts)
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, fmt.Sprintf("attempt %d", test.attempts), response.Body.String())
		})
	}
}

func TestBuffer_chunkedEncodingSuccess(t *testing.T) {
	var (
		reqBody       string
		contentLength int64
	)

	srv := testutils.NewServer(t, func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		reqBody = string(body)
		contentLength = req.ContentLength

		_, _ = w.Write([]byte("hello"))
	})

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	conn, err := net.Dial("tcp", testutils.MustParseRequestURI(proxy.URL).Host)
	require.NoError(t, err)

	_, _ = fmt.Fprintf(conn, "POST / HTTP/1.1\r\nHost: 127.0.0.1:8080\r\nTransfer-Encoding: chunked\r\n\r\n4\r\ntest\r\n5\r\ntest1\r\n5\r\ntest2\r\n0\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')
	require.NoError(t, err)

	assert.Equal(t, "testtest1test2", reqBody)
	assert.Equal(t, "HTTP/1.1 200 OK\r\n", status)
	assert.EqualValues(t, len(reqBody), contentLength)
}

func TestBuffer_chunkedEncodingLimitReached(t *testing.T) {
	srv := testutils.NewServerWithResponse(t, "hello")

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr, MemRequestBodyBytes(4), MaxRequestBodyBytes(8))
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	conn, err := net.Dial("tcp", testutils.MustParseRequestURI(proxy.URL).Host)
	require.NoError(t, err)

	_, _ = fmt.Fprint(conn, "POST / HTTP/1.1\r\nHost: 127.0.0.1:8080\r\nTransfer-Encoding: chunked\r\n\r\n4\r\ntest\r\n5\r\ntest1\r\n5\r\ntest2\r\n0\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')
	require.NoError(t, err)

	assert.Equal(t, "HTTP/1.1 413 Request Entity Too Large\r\n", status)
}

func TestBuffer_chunkedResponse(t *testing.T) {
	srv := testutils.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		h := w.(http.Hijacker)
		conn, _, _ := h.Hijack()
		_, _ = fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n4\r\ntest\r\n5\r\ntest1\r\n5\r\ntest2\r\n0\r\n\r\n")
		_ = conn.Close()
	})

	fwd := forward.New(false)

	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, "testtest1test2", string(body))
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, strconv.Itoa(len("testtest1test2")), re.Header.Get("Content-Length"))
}

func TestBuffer_emptyChunkedResponse(t *testing.T) {
	srv := testutils.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		h := w.(http.Hijacker)
		conn, _, _ := h.Hijack()
		_, _ = fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n")
		_ = conn.Close()
	})

	fwd := forward.New(false)

	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Empty(t, body)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

func TestBuffer_requestLimitReached(t *testing.T) {
	srv := testutils.NewServerWithResponse(t, "hello")

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr, MaxRequestBodyBytes(4))
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, _, err := testutils.Get(proxy.URL, testutils.Body("this request is too long"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusRequestEntityTooLarge, re.StatusCode)
}

func TestBuffer_responseLimitReached(t *testing.T) {
	cases := []struct {
		name                 string
		body                 string
		maxResponseBodyBytes int64
	}{
		{
			name:                 "small limit with body larger than max response bytes",
			body:                 "hello, this response is too large",
			maxResponseBodyBytes: 4,
		},
		{
			name:                 "small limit with body larger than 32768 bytes",
			body:                 strings.Repeat("A", 32769),
			maxResponseBodyBytes: 4,
		},
		{
			name:                 "larger limit with body larger than 32768 bytes",
			body:                 strings.Repeat("A", 32769),
			maxResponseBodyBytes: 2000,
		},
		{
			name:                 "larger limit with body larger than 32768 + 1999 bytes",
			body:                 strings.Repeat("A", 32769+1999),
			maxResponseBodyBytes: 2000,
		},
		{
			name:                 "larger limit with body larger than 32768 + 2000 bytes",
			body:                 strings.Repeat("A", 32769+2000),
			maxResponseBodyBytes: 2000,
		},
		{
			name:                 "larger limit with body larger than 65536 + 1999 bytes",
			body:                 strings.Repeat("A", 65537+1999),
			maxResponseBodyBytes: 2000,
		},
		{
			name:                 "larger limit with body larger than 65536 + 2000 bytes",
			body:                 strings.Repeat("A", 65537+2000),
			maxResponseBodyBytes: 2000,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := testutils.NewServerWithResponse(t, tc.body)

			// forwarder will proxy the request to whatever destination
			fwd := forward.New(false)

			// this is our redirect to server
			rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				req.URL = testutils.MustParseRequestURI(srv.URL)
				fwd.ServeHTTP(w, req)
			})

			// stream handler will forward requests to redirect
			st, err := New(rdr, MaxResponseBodyBytes(tc.maxResponseBodyBytes))
			require.NoError(t, err)

			proxy := httptest.NewServer(st)
			t.Cleanup(proxy.Close)

			re, _, err := testutils.Get(proxy.URL)
			require.NoError(t, err)
			assert.Equal(t, http.StatusInternalServerError, re.StatusCode)
		})
	}
}

func TestBuffer_fileStreamingResponse(t *testing.T) {
	srv := testutils.NewServerWithResponse(t, "hello, this response is too large to fit in memory")

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr, MemResponseBodyBytes(4))
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello, this response is too large to fit in memory", string(body))
}

func TestBuffer_customErrorHandler(t *testing.T) {
	srv := testutils.NewServerWithResponse(t, "hello, this response is too large")

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	errHandler := utils.ErrorHandlerFunc(func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(http.StatusText(http.StatusTeapot)))
	})
	st, err := New(rdr, MaxResponseBodyBytes(4), ErrorHandler(errHandler))
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, re.StatusCode)
}

func TestBuffer_notModified(t *testing.T) {
	srv := testutils.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	})

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotModified, re.StatusCode)
}

func TestBuffer_noBody(t *testing.T) {
	srv := testutils.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

// Make sure that stream handler preserves TLS settings.
func TestBuffer_preservesTLS(t *testing.T) {
	srv := testutils.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	var cs *tls.ConnectionState
	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cs = req.TLS
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewUnstartedServer(st)
	proxy.StartTLS()
	t.Cleanup(proxy.Close)

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.NotNil(t, cs)
}

func TestBuffer_notNilBody(t *testing.T) {
	srv := testutils.NewServerWithResponse(t, "hello")

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		// During a request check if the request body is no nil before sending to the next middleware
		// Because we can send a POST request without body
		assert.NotNil(t, req.Body)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr, MaxRequestBodyBytes(10))
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))

	re, body, err = testutils.Post(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))
}

func TestBuffer_GRPC_ErrorResponse(t *testing.T) {
	srv := testutils.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Grpc-Status", "10" /* ABORTED */)
		w.WriteHeader(http.StatusOK)

		// To skip the "Content-Length" header.
		w.(http.Flusher).Flush()
	})

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Empty(t, body)
}

func TestBuffer_GRPC_OKResponse(t *testing.T) {
	srv := testutils.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Grpc-Status", "0" /* OK */)
		_, _ = w.Write([]byte("grpc-body"))

		// To skip the "Content-Length" header.
		w.(http.Flusher).Flush()
	})

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "grpc-body", string(body))
}

func TestBuffer_disableRequestBuffer(t *testing.T) {
	var (
		reqBody          string
		contentLength    int64
		actuallyBuffered bool
	)

	srv := testutils.NewServer(t, func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		reqBody = string(body)
		contentLength = req.ContentLength
		// When buffering is disabled, chunked requests should preserve their transfer encoding, and have no content-length.
		actuallyBuffered = contentLength > 0 || len(req.TransferEncoding) == 0
		_, _ = w.Write([]byte("response"))
	})

	fwd := forward.New(false)
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// buffer with disabled request buffering.
	st, err := New(rdr, DisableRequestBuffer())
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	// Send a chunked request - when buffering is disabled, it should remain chunked.
	conn, err := net.Dial("tcp", testutils.MustParseRequestURI(proxy.URL).Host)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, _ = fmt.Fprintf(conn, "POST / HTTP/1.1\r\nHost: %s\r\nTransfer-Encoding: chunked\r\n\r\n4\r\ntest\r\n0\r\n\r\n", testutils.MustParseRequestURI(proxy.URL).Host)
	status, err := bufio.NewReader(conn).ReadString('\n')
	require.NoError(t, err)

	assert.Equal(t, "HTTP/1.1 200 OK\r\n", status)
	assert.Equal(t, "test", reqBody)
	// When buffering is disabled, chunked encoding should be preserved (not converted to Content-Length).
	assert.False(t, actuallyBuffered, "Request should not have been buffered")
	assert.Equal(t, int64(-1), contentLength, "Content-Length should be -1 for chunked requests when not buffered")
}

func TestBuffer_disableResponseBuffer(t *testing.T) {
	largeResponseBody := strings.Repeat("A", 1000)

	srv := testutils.NewServerWithResponse(t, largeResponseBody)

	fwd := forward.New(false)
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// buffer with a small max response size.
	st, err := New(rdr, MaxResponseBodyBytes(4))
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	resp, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	// Response should not pass through as it exceeds the limit.
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// buffer with disabled response buffering and a small max response size.
	st, err = New(rdr, DisableResponseBuffer(), MaxResponseBodyBytes(4))
	require.NoError(t, err)

	proxy2 := httptest.NewServer(st)
	t.Cleanup(proxy2.Close)

	resp2, body, err := testutils.Get(proxy2.URL)
	require.NoError(t, err)
	// Response should pass through even though it exceeds the limit, because buffering has been disabled.
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Equal(t, largeResponseBody, string(body))
}

func TestBuffer_responseControllerFlush(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		assert.ErrorIs(t, http.NewResponseController(w).Flush(), http.ErrNotSupported)
		_, _ = w.Write([]byte("unavailable"))
	})
	st, err := New(handler)
	require.NoError(t, err)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)

	st.ServeHTTP(response, request)

	assert.False(t, response.Flushed)
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.Equal(t, "unavailable", response.Body.String())
}

func TestBuffer_responseControllerUnwrap(t *testing.T) {
	deadline := time.Now().Add(time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, http.NewResponseController(w).SetWriteDeadline(deadline))
	})
	st, err := New(handler)
	require.NoError(t, err)

	response := &controllerRecorder{ResponseRecorder: httptest.NewRecorder()}
	request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)

	st.ServeHTTP(response, request)

	assert.Equal(t, deadline, response.writeDeadline)
}

func TestBuffer_responseControllerHijack(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _, err := http.NewResponseController(w).Hijack()
		require.NoError(t, err)
	})
	st, err := New(handler)
	require.NoError(t, err)

	recorder := &controllerRecorder{ResponseRecorder: httptest.NewRecorder()}
	request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)

	st.ServeHTTP(unwrapper{recorder}, request)

	assert.True(t, recorder.hijacked)
}

func TestBuffer_cleanFiles(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 2048)

	for _, test := range []struct {
		name     string
		method   string
		options  []Option
		retry    bool
		abort    bool
		code     int
		body     []byte
		attempts int
	}{
		{name: "success", method: http.MethodGet, code: http.StatusOK, body: payload, attempts: 1},
		{name: "over_limit", method: http.MethodGet, options: []Option{MaxResponseBodyBytes(1536)}, code: http.StatusInternalServerError, attempts: 1},
		{name: "aborted_handler", method: http.MethodGet, abort: true, attempts: 1},
		{name: "head", method: http.MethodHead, code: http.StatusOK, attempts: 1},
		{name: "retry", method: http.MethodGet, retry: true, code: http.StatusOK, body: payload, attempts: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()

			attempts := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts++

				if test.retry && attempts == 1 {
					w.WriteHeader(http.StatusBadGateway)
				}

				for chunk := range slices.Chunk(payload, 512) {
					_, _ = w.Write(chunk)
				}

				require.NotEmpty(t, bufferingFiles(t, dir), "the response must spill to disk")

				if test.abort {
					panic(http.ErrAbortHandler)
				}
			})

			options := append(
				[]Option{tempBufferDir(dir), MemResponseBodyBytes(1024)},
				test.options...,
			)

			if test.retry {
				options = append(options, Retry(`ResponseCode() == 502 && Attempts() <= 1`))
			}

			st, err := New(handler, options...)
			require.NoError(t, err)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, "http://example.com/", nil)

			if test.abort {
				assert.PanicsWithValue(t, http.ErrAbortHandler, func() { st.ServeHTTP(recorder, request) })
			} else {
				st.ServeHTTP(recorder, request)

				assert.Equal(t, test.code, recorder.Code)
			}

			if test.code == http.StatusOK {
				assert.Equal(t, string(test.body), recorder.Body.String())
			}

			assert.Equal(t, test.attempts, attempts)
			assert.Empty(t, bufferingFiles(t, dir))
		})
	}
}

func TestBuffer_cleanFiles_clientDisconnect(t *testing.T) {
	dir := t.TempDir()

	upstream := testutils.NewServer(t, func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 2048))
		_ = http.NewResponseController(w).Flush()

		// Never complete the response: the proxy is still buffering it when the client leaves.
		<-req.Context().Done()
	})

	fwd := forward.New(false)

	st, err := New(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.URL = testutils.MustParseRequestURI(upstream.URL)
			fwd.ServeHTTP(w, req)
		}),
		MemResponseBodyBytes(1024), tempBufferDir(dir),
	)
	require.NoError(t, err)

	served := make(chan struct{})

	proxy := testutils.NewServer(t, func(w http.ResponseWriter, req *http.Request) {
		defer close(served)

		st.ServeHTTP(w, req)
	})

	ctx, cancel := gocontext.WithCancel(t.Context())
	t.Cleanup(cancel)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, proxy.URL, nil)
	require.NoError(t, err)

	result := make(chan error, 1)

	go func() {
		response, errDo := http.DefaultClient.Do(request)
		if errDo == nil {
			_ = response.Body.Close()
		}

		result <- errDo
	}()

	require.Eventually(t, func() bool {
		return len(bufferingFiles(t, dir)) > 0
	}, 5*time.Second, 10*time.Millisecond, "the response must spill to disk")

	cancel()
	require.ErrorIs(t, <-result, gocontext.Canceled)

	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("the proxy handler did not finish after the client disconnected")
	}

	assert.Empty(t, bufferingFiles(t, dir))
}

func bufferingFiles(t *testing.T, dir string) []os.DirEntry {
	t.Helper()

	files, err := os.ReadDir(dir)
	require.NoError(t, err)

	return files
}

func tempBufferDir(dir string) Option {
	return func(b *Buffer) error {
		b.tempDir = dir

		return nil
	}
}

// controllerRecorder records the http.ResponseController calls reaching it.
type controllerRecorder struct {
	*httptest.ResponseRecorder

	writeDeadline time.Time
	hijacked      bool
}

func (c *controllerRecorder) SetWriteDeadline(deadline time.Time) error {
	c.writeDeadline = deadline
	return nil
}

func (c *controllerRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c.hijacked = true
	return nil, nil, nil
}

// unwrapper only exposes the wrapped ResponseWriter through Unwrap.
type unwrapper struct {
	http.ResponseWriter
}

func (u unwrapper) Unwrap() http.ResponseWriter {
	return u.ResponseWriter
}
