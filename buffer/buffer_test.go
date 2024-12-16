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

func TestBuffer_chunkedEncodingSuccess(t *testing.T) {
	var reqBody string
	var contentLength int64
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
