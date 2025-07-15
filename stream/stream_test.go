package stream

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/forward"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
	"github.com/vulcand/oxy/v2/testutils"
)

func TestStream_simple(t *testing.T) {
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

func TestStream_chunkedEncodingSuccess(t *testing.T) {
	var (
		reqBody       string
		contentLength int64
	)

	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		reqBody = string(body)
		contentLength = req.ContentLength

		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			panic("expected http.ResponseWriter to be an http.Flusher")
		}

		_, _ = fmt.Fprint(w, "Response")

		flusher.Flush()
		clock.Sleep(500 * clock.Millisecond)

		_, _ = fmt.Fprint(w, "in")

		flusher.Flush()
		clock.Sleep(500 * clock.Millisecond)

		_, _ = fmt.Fprint(w, "Chunks")

		flusher.Flush()
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

	_, _ = fmt.Fprint(conn, "POST / HTTP/1.1\r\nHost: 127.0.0.1\r\nTransfer-Encoding: chunked\r\n\r\n4\r\ntest\r\n5\r\ntest1\r\n5\r\ntest2\r\n0\r\n\r\n")
	reader := bufio.NewReader(conn)

	status, err := reader.ReadString('\n')
	require.NoError(t, err)

	_, err = reader.ReadString('\n') // content type
	require.NoError(t, err)
	_, err = reader.ReadString('\n') // Date
	require.NoError(t, err)
	transferEncoding, err := reader.ReadString('\n')
	require.NoError(t, err)

	assert.Equal(t, "Transfer-Encoding: chunked\r\n", transferEncoding)
	assert.Equal(t, int64(-1), contentLength)
	assert.Equal(t, "testtest1test2", reqBody)
	assert.Equal(t, "HTTP/1.1 200 OK\r\n", status)
}

func TestStream_requestLimitReached(t *testing.T) {
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

	re, _, err := testutils.Get(proxy.URL, testutils.Body("this request is too long"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

func TestStream_responseLimitReached(t *testing.T) {
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
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

func TestStream_fileStreamingResponse(t *testing.T) {
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
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello, this response is too large to fit in memory", string(body))
}

func TestStream_customErrorHandler(t *testing.T) {
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

	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	t.Cleanup(proxy.Close)

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

func TestStream_notModified(t *testing.T) {
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

func TestStream_noBody(t *testing.T) {
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
func TestStream_preservesTLS(t *testing.T) {
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
