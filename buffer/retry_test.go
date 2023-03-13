package buffer

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/forward"
	"github.com/vulcand/oxy/v2/roundrobin"
	"github.com/vulcand/oxy/v2/testutils"
)

func TestSuccess(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(srv.Close)

	lb, rt := newBufferMiddleware(t, `IsNetworkError() && Attempts() <= 2`)

	proxy := httptest.NewServer(rt)
	t.Cleanup(proxy.Close)

	require.NoError(t, lb.UpsertServer(testutils.ParseURI(srv.URL)))

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))
}

func TestRetryOnError(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(srv.Close)

	lb, rt := newBufferMiddleware(t, `IsNetworkError() && Attempts() <= 2`)

	proxy := httptest.NewServer(rt)
	t.Cleanup(proxy.Close)

	require.NoError(t, lb.UpsertServer(testutils.ParseURI("http://localhost:64321")))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI(srv.URL)))

	re, body, err := testutils.Get(proxy.URL, testutils.Body("some request parameters"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))
}

func TestRetryExceedAttempts(t *testing.T) {
	server := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(server.Close)

	countDeadCalls := atomic.Int32{}

	// uses 20 to have a higher value than DefaultMaxRetryAttempts (10)
	lb, rt := newBufferMiddleware(t, `IsNetworkError() && Attempts() <= 20`)

	proxy := httptest.NewServer(rt)
	t.Cleanup(proxy.Close)

	// creates more dead server than the expected number of retries (20)
	for i := 0; i <= 30; i++ {
		deadServer := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
			countDeadCalls.Add(1)
			w.WriteHeader(http.StatusBadGateway)
		})
		t.Cleanup(deadServer.Close)

		require.NoError(t, lb.UpsertServer(testutils.ParseURI(deadServer.URL)))
	}

	require.NoError(t, lb.UpsertServer(testutils.ParseURI(server.URL)))

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, re.StatusCode)

	assert.Equal(t, int32(21), countDeadCalls.Load())
}

func newBufferMiddleware(t *testing.T, p string) (*roundrobin.RoundRobin, *Buffer) {
	t.Helper()

	// forwarder will proxy the request to whatever destination
	fwd := forward.New(false)

	// load balancer will round robin request
	lb, err := roundrobin.New(fwd)
	require.NoError(t, err)

	// stream handler will forward requests to redirect, make sure it uses files
	st, err := New(lb, Retry(p), MemRequestBodyBytes(1))
	require.NoError(t, err)

	return lb, st
}
