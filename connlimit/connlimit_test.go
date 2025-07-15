package connlimit

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/testutils"
	"github.com/vulcand/oxy/v2/utils"
)

// We've hit the limit and were able to proceed once the request has completed.
func TestConnLimiter_hitLimitAndRelease(t *testing.T) {
	wait := make(chan bool)
	proceed := make(chan bool)
	finish := make(chan bool)

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Logf("%v", req.Header)

		if req.Header.Get("Wait") != "" {
			proceed <- true

			<-wait
		}

		_, _ = w.Write([]byte("hello"))
	})

	cl, err := New(handler, headerLimit, 1)
	require.NoError(t, err)

	srv := httptest.NewServer(cl)
	t.Cleanup(srv.Close)

	go func() {
		re, _, errGet := testutils.Get(srv.URL, testutils.Header("Limit", "a"), testutils.Header("wait", "yes"))
		require.NoError(t, errGet)
		assert.Equal(t, http.StatusOK, re.StatusCode)

		finish <- true
	}()

	<-proceed

	re, _, err := testutils.Get(srv.URL, testutils.Header("Limit", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, re.StatusCode)

	// request from another source succeeds
	re, _, err = testutils.Get(srv.URL, testutils.Header("Limit", "b"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	// Once the first request finished, next one succeeds
	close(wait)
	<-finish

	re, _, err = testutils.Get(srv.URL, testutils.Header("Limit", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

// We've hit the limit and were able to proceed once the request has completed.
func TestConnLimiter_customHandlers(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	errHandler := utils.ErrorHandlerFunc(func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(http.StatusText(http.StatusTeapot)))
	})

	l, err := New(handler, headerLimit, 0, ErrorHandler(errHandler))
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	t.Cleanup(srv.Close)

	re, _, err := testutils.Get(srv.URL, testutils.Header("Limit", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, re.StatusCode)
}

// We've hit the limit and were able to proceed once the request has completed.
func TestConnLimiter_faultyExtract(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	l, err := New(handler, faultyExtract, 1)
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	t.Cleanup(srv.Close)

	re, _, err := testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, re.StatusCode)
}

func headerLimiter(req *http.Request) (string, int64, error) {
	return req.Header.Get("Limit"), 1, nil
}

func faultyExtractor(_ *http.Request) (string, int64, error) {
	return "", -1, errors.New("oops")
}

var headerLimit = utils.ExtractorFunc(headerLimiter)

var faultyExtract = utils.ExtractorFunc(faultyExtractor)
