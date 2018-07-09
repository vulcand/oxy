package stream

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/testutils"
)

type noOpNextHTTPHandler struct{}

func (n noOpNextHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

type noOpIoWriter struct{}

func (n noOpIoWriter) Write(bytes []byte) (int, error) {
	return len(bytes), nil
}

func TestSimple(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello"))
	})
	defer srv.Close()

	// forwarder will proxy the request to whatever destination
	fwd, err := forward.New(forward.Stream(true))
	require.NoError(t, err)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	defer proxy.Close()

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))
}

func TestChunkedEncodingSuccess(t *testing.T) {
	var reqBody string
	var contentLength int64
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		require.NoError(t, err)
		reqBody = string(body)
		contentLength = req.ContentLength

		w.WriteHeader(200)
		flusher, ok := w.(http.Flusher)
		if !ok {
			panic("expected http.ResponseWriter to be an http.Flusher")
		}
		fmt.Fprint(w, "Response")
		flusher.Flush()
		time.Sleep(time.Duration(500) * time.Millisecond)
		fmt.Fprint(w, "in")
		flusher.Flush()
		time.Sleep(time.Duration(500) * time.Millisecond)
		fmt.Fprint(w, "Chunks")
		flusher.Flush()
	})
	defer srv.Close()

	// forwarder will proxy the request to whatever destination
	fwd, err := forward.New(forward.Stream(true))
	require.NoError(t, err)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	defer proxy.Close()

	conn, err := net.Dial("tcp", testutils.ParseURI(proxy.URL).Host)
	require.NoError(t, err)
	fmt.Fprint(conn, "POST / HTTP/1.1\r\nHost: 127.0.0.1\r\nTransfer-Encoding: chunked\r\n\r\n4\r\ntest\r\n5\r\ntest1\r\n5\r\ntest2\r\n0\r\n\r\n")
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

func TestRequestLimitReached(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello"))
	})
	defer srv.Close()

	// forwarder will proxy the request to whatever destination
	fwd, err := forward.New(forward.Stream(true))
	require.NoError(t, err)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	defer proxy.Close()

	re, _, err := testutils.Get(proxy.URL, testutils.Body("this request is too long"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

func TestResponseLimitReached(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello, this response is too large"))
	})
	defer srv.Close()

	// forwarder will proxy the request to whatever destination
	fwd, err := forward.New(forward.Stream(true))
	require.NoError(t, err)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	defer proxy.Close()

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

func TestFileStreamingResponse(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello, this response is too large to fit in memory"))
	})
	defer srv.Close()

	// forwarder will proxy the request to whatever destination
	fwd, err := forward.New(forward.Stream(true))
	require.NoError(t, err)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	defer proxy.Close()

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello, this response is too large to fit in memory", string(body))
}

func TestCustomErrorHandler(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello, this response is too large"))
	})
	defer srv.Close()

	// forwarder will proxy the request to whatever destination
	fwd, err := forward.New(forward.Stream(true))
	require.NoError(t, err)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	defer proxy.Close()

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

func TestNotModified(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	})
	defer srv.Close()

	// forwarder will proxy the request to whatever destination
	fwd, err := forward.New(forward.Stream(true))
	require.NoError(t, err)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	defer proxy.Close()

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotModified, re.StatusCode)
}

func TestNoBody(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	// forwarder will proxy the request to whatever destination
	fwd, err := forward.New(forward.Stream(true))
	require.NoError(t, err)

	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.ParseURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewServer(st)
	defer proxy.Close()

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

// Make sure that stream handler preserves TLS settings
func TestPreservesTLS(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	defer srv.Close()

	// forwarder will proxy the request to whatever destination
	fwd, err := forward.New(forward.Stream(true))
	require.NoError(t, err)

	var cs *tls.ConnectionState
	// this is our redirect to server
	rdr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cs = req.TLS
		req.URL = testutils.ParseURI(srv.URL)
		fwd.ServeHTTP(w, req)
	})

	// stream handler will forward requests to redirect
	st, err := New(rdr)
	require.NoError(t, err)

	proxy := httptest.NewUnstartedServer(st)
	proxy.StartTLS()
	defer proxy.Close()

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	assert.NotNil(t, cs)
}

func BenchmarkLoggingDebugLevel(b *testing.B) {
	streamer, _ := New(noOpNextHTTPHandler{})

	log.SetLevel(log.DebugLevel)
	log.SetOutput(&noOpIoWriter{}) // Make sure we don't emit a bunch of stuff on screen

	for i := 0; i < b.N; i++ {
		heavyServeHTTPLoad(streamer)
	}
}

func BenchmarkLoggingInfoLevel(b *testing.B) {
	streamer, _ := New(noOpNextHTTPHandler{})

	log.SetLevel(log.InfoLevel)
	log.SetOutput(&noOpIoWriter{}) // Make sure we don't emit a bunch of stuff on screen

	for i := 0; i < b.N; i++ {
		heavyServeHTTPLoad(streamer)
	}
}

func heavyServeHTTPLoad(handler http.Handler) {
	w := httptest.NewRecorder()
	r := &http.Request{}
	handler.ServeHTTP(w, r)
}
