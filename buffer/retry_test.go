package buffer

import (
	stdcontext "context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/forward"
	"github.com/vulcand/oxy/v2/roundrobin"
	"github.com/vulcand/oxy/v2/testutils"
)

func TestBuffer_retryStopsWhenContextEnds(t *testing.T) {
	for _, name := range []string{"canceled", "deadline", "active"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := stdcontext.WithCancel(t.Context())
			t.Cleanup(cancel)

			if name == "deadline" {
				var deadlineCancel stdcontext.CancelFunc

				ctx, deadlineCancel = stdcontext.WithDeadline(ctx, time.Now().Add(-time.Second))
				t.Cleanup(deadlineCancel)
			}

			attempts := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts++

				if name == "canceled" {
					cancel()
				}

				w.Header().Set("X-Upstream", "unavailable")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte("upstream unavailable"))
			})
			st, err := New(handler, Retry(`IsNetworkError() && Attempts() < 3`))
			require.NoError(t, err)

			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil).WithContext(ctx)
			st.ServeHTTP(response, request)

			wantAttempts := 1
			if name == "active" {
				wantAttempts = 3
			}

			assert.Equal(t, wantAttempts, attempts)
			assert.Equal(t, http.StatusBadGateway, response.Code)
			assert.Equal(t, "unavailable", response.Header().Get("X-Upstream"))
			assert.Equal(t, "upstream unavailable", response.Body.String())
		})
	}
}

func TestBuffer_success(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(srv.Close)

	lb, rt := newBufferMiddleware(t, `IsNetworkError() && Attempts() <= 2`)

	proxy := httptest.NewServer(rt)
	t.Cleanup(proxy.Close)

	require.NoError(t, lb.UpsertServer(testutils.MustParseRequestURI(srv.URL)))

	re, body, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))
}

func TestBuffer_retryOnError(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(srv.Close)

	lb, rt := newBufferMiddleware(t, `IsNetworkError() && Attempts() <= 2`)

	proxy := httptest.NewServer(rt)
	t.Cleanup(proxy.Close)

	require.NoError(t, lb.UpsertServer(testutils.MustParseRequestURI("http://localhost:64321")))
	require.NoError(t, lb.UpsertServer(testutils.MustParseRequestURI(srv.URL)))

	re, body, err := testutils.Get(proxy.URL, testutils.Body("some request parameters"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))
}

func TestBuffer_retryExceedAttempts(t *testing.T) {
	srv := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	t.Cleanup(srv.Close)

	lb, rt := newBufferMiddleware(t, `IsNetworkError() && Attempts() <= 2`)

	proxy := httptest.NewServer(rt)
	t.Cleanup(proxy.Close)

	require.NoError(t, lb.UpsertServer(testutils.MustParseRequestURI("http://localhost:64321")))
	require.NoError(t, lb.UpsertServer(testutils.MustParseRequestURI("http://localhost:64322")))
	require.NoError(t, lb.UpsertServer(testutils.MustParseRequestURI("http://localhost:64323")))
	require.NoError(t, lb.UpsertServer(testutils.MustParseRequestURI(srv.URL)))

	re, _, err := testutils.Get(proxy.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, re.StatusCode)
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
