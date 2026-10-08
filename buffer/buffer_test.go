package buffer

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/forward"
	"github.com/vulcand/oxy/v2/testutils"
	"github.com/vulcand/oxy/v2/utils"
)

func TestBuffer_simple(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(srv.Close)

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

	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		reqBody = string(body)
		contentLength = req.ContentLength
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		h := w.(http.Hijacker)
		conn, _, _ := h.Hijack()
		_, _ = fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n4\r\ntest\r\n5\r\ntest1\r\n5\r\ntest2\r\n0\r\n\r\n")
		_ = conn.Close()
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		h := w.(http.Hijacker)
		conn, _, _ := h.Hijack()
		_, _ = fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n")
		_ = conn.Close()
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(srv.Close)

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
			srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			})
			t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello, this response is too large to fit in memory"))
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello, this response is too large"))
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Grpc-Status", "10" /* ABORTED */)
		w.WriteHeader(http.StatusOK)

		// To skip the "Content-Length" header.
		w.(http.Flusher).Flush()
	})
	t.Cleanup(srv.Close)

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Grpc-Status", "0" /* OK */)
		_, _ = w.Write([]byte("grpc-body"))

		// To skip the "Content-Length" header.
		w.(http.Flusher).Flush()
	})
	t.Cleanup(srv.Close)

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

	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		reqBody = string(body)
		contentLength = req.ContentLength
		// When buffering is disabled, chunked requests should preserve their transfer encoding, and have no content-length.
		actuallyBuffered = contentLength > 0 || len(req.TransferEncoding) == 0
		_, _ = w.Write([]byte("response"))
	})
	t.Cleanup(srv.Close)

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
	t.Cleanup(func() {
		_ = conn.Close()
	})

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
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(largeResponseBody))
	})
	t.Cleanup(srv.Close)

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
