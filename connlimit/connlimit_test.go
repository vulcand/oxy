package connlimit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
	"github.com/vulcand/oxy/utils"
)

// We've hit the limit and were able to proceed once the request has completed
func TestHitLimitAndRelease(t *testing.T) {
	wait := make(chan bool)
	proceed := make(chan bool)
	finish := make(chan bool)

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Println(req.Header)
		if req.Header.Get("Wait") != "" {
			proceed <- true
			<-wait
		}
		w.Write([]byte("hello"))
	})

	cl, err := New(handler, headerLimit, 1)
	require.NoError(t, err)

	srv := httptest.NewServer(cl)
	defer srv.Close()

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

// We've hit the limit and were able to proceed once the request has completed
func TestCustomHandlers(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello"))
	})

	errHandler := utils.ErrorHandlerFunc(func(w http.ResponseWriter, req *http.Request, err error) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(http.StatusText(http.StatusTeapot)))
	})

	l, err := New(handler, headerLimit, 0, ErrorHandler(errHandler))
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL, testutils.Header("Limit", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, re.StatusCode)
}

// We've hit the limit and were able to proceed once the request has completed
func TestFaultyExtract(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello"))
	})

	l, err := New(handler, faultyExtract, 1)
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, re.StatusCode)
}

func headerLimiter(req *http.Request) (string, int64, error) {
	return req.Header.Get("Limit"), 1, nil
}

func faultyExtractor(_ *http.Request) (string, int64, error) {
	return "", -1, fmt.Errorf("oops")
}

var headerLimit = utils.ExtractorFunc(headerLimiter)
var faultyExtract = utils.ExtractorFunc(faultyExtractor)
