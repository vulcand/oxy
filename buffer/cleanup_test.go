package buffer

import (
	"bytes"
	gocontext "context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/forward"
	"github.com/vulcand/oxy/v2/testutils"
)

// spillDir points temporary files to an empty directory, so a test can check
// that response buffers spilled to disk are removed.
func spillDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)

	return dir
}

func spillFiles(t *testing.T, dir string) []os.DirEntry {
	t.Helper()

	files, err := os.ReadDir(dir)
	require.NoError(t, err)

	return files
}

func TestBuffer_responseSpillFileRemoved(t *testing.T) {
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
			dir := spillDir(t)

			attempts := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts++

				if test.retry && attempts == 1 {
					w.WriteHeader(http.StatusBadGateway)
				}

				for chunk := range slices.Chunk(payload, 512) {
					_, _ = w.Write(chunk)
				}

				require.NotEmpty(t, spillFiles(t, dir), "the response must spill to disk")

				if test.abort {
					panic(http.ErrAbortHandler)
				}
			})

			options := append([]Option{MemResponseBodyBytes(1024)}, test.options...)
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
			assert.Empty(t, spillFiles(t, dir))
		})
	}
}

// A client that disconnects while the upstream response is being buffered makes
// httputil.ReverseProxy abort the handler (https://github.com/traefik/traefik/issues/11372).
func TestBuffer_clientDisconnectRemovesResponseSpillFile(t *testing.T) {
	dir := spillDir(t)

	upstream := testutils.NewHandler(func(w http.ResponseWriter, req *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 2048))
		_ = http.NewResponseController(w).Flush()

		// Never complete the response: the proxy is still buffering it when the client leaves.
		<-req.Context().Done()
	})
	t.Cleanup(upstream.Close)

	fwd := forward.New(false)

	st, err := New(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.URL = testutils.MustParseRequestURI(upstream.URL)
		fwd.ServeHTTP(w, req)
	}), MemResponseBodyBytes(1024))
	require.NoError(t, err)

	served := make(chan struct{})

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer close(served)

		st.ServeHTTP(w, req)
	}))
	t.Cleanup(proxy.Close)

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
		return len(spillFiles(t, dir)) > 0
	}, 5*time.Second, 10*time.Millisecond, "the response must spill to disk")

	cancel()
	require.ErrorIs(t, <-result, gocontext.Canceled)

	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("the proxy handler did not finish after the client disconnected")
	}

	assert.Empty(t, spillFiles(t, dir))
}
