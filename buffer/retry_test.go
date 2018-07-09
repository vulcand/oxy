package buffer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/roundrobin"
	"github.com/vulcand/oxy/testutils"
)

func TestSuccess(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello"))
	})
	defer srv.Close()

	lb, rt := newBufferMiddleware(t, `IsNetworkError() && Attempts() <= 2`)

	proxy := httptest.NewServer(rt)
	defer proxy.Close()

	require.NoError(t, lb.UpsertServer(testutils.ParseURI(srv.URL)))

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))
}

func TestRetryOnError(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello"))
	})
	defer srv.Close()

	lb, rt := newBufferMiddleware(t, `IsNetworkError() && Attempts() <= 2`)

	proxy := httptest.NewServer(rt)
	defer proxy.Close()

	require.NoError(t, lb.UpsertServer(testutils.ParseURI("http://localhost:64321")))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI(srv.URL)))

	re, body, err := testutils.Get(proxy.URL, testutils.Body("some request parameters"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))
}

func TestRetryExceedAttempts(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello"))
	})
	defer srv.Close()

	lb, rt := newBufferMiddleware(t, `IsNetworkError() && Attempts() <= 2`)

	proxy := httptest.NewServer(rt)
	defer proxy.Close()

	require.NoError(t, lb.UpsertServer(testutils.ParseURI("http://localhost:64321")))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI("http://localhost:64322")))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI("http://localhost:64323")))
	require.NoError(t, lb.UpsertServer(testutils.ParseURI(srv.URL)))

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, re.StatusCode)
}

func newBufferMiddleware(t *testing.T, p string) (*roundrobin.RoundRobin, *Buffer) {
	// forwarder will proxy the request to whatever destination
	fwd, err := forward.New()
	require.NoError(t, err)

	// load balancer will round robin request
	lb, err := roundrobin.New(fwd)
	require.NoError(t, err)

	// stream handler will forward requests to redirect, make sure it uses files
	st, err := New(lb, Retry(p), MemRequestBodyBytes(1))
	require.NoError(t, err)

	return lb, st
}
