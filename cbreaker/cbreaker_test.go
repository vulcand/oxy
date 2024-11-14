package cbreaker

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
	"github.com/vulcand/oxy/v2/memmetrics"
	"github.com/vulcand/oxy/v2/testutils"
)

const triggerNetRatio = `NetworkErrorRatio() > 0.5`

func TestCircuitBreaker_standbyCycle(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	cb, err := New(handler, triggerNetRatio)
	require.NoError(t, err)

	srv := httptest.NewServer(cb)
	t.Cleanup(srv.Close)

	re, body, err := testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
	assert.Equal(t, "hello", string(body))
}

func TestCircuitBreaker_fullCycle(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	testutils.FreezeTime(t)

	cb, err := New(handler, triggerNetRatio)
	require.NoError(t, err)

	srv := httptest.NewServer(cb)
	t.Cleanup(srv.Close)

	re, _, err := testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	cb.metrics = statsNetErrors(0.6)
	clock.Advance(defaultCheckPeriod + clock.Millisecond)
	_, _, err = testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, cbState(stateTripped), cb.state)

	// Some time has passed, but we are still in trapped state.
	clock.Advance(9 * clock.Second)
	re, _, err = testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, re.StatusCode)
	assert.Equal(t, cbState(stateTripped), cb.state)

	// We should be in recovering state by now
	clock.Advance(clock.Second*1 + clock.Millisecond)
	re, _, err = testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, re.StatusCode)
	assert.Equal(t, cbState(stateRecovering), cb.state)

	// 5 seconds after we should be allowing some requests to pass
	clock.Advance(5 * clock.Second)
	allowed := 0
	for i := 0; i < 100; i++ {
		re, _, err = testutils.Get(srv.URL)
		if re.StatusCode == http.StatusOK && err == nil {
			allowed++
		}
	}
	assert.NotEqual(t, 0, allowed)

	// After some time, all is good and we should be in stand by mode again
	clock.Advance(5*clock.Second + clock.Millisecond)
	re, _, err = testutils.Get(srv.URL)
	assert.Equal(t, cbState(stateStandby), cb.state)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

func TestCircuitBreaker_redirectWithPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	fallbackRedirectPath, err := NewRedirectFallback(Redirect{
		URL:          "http://localhost:6000",
		PreservePath: true,
	})
	require.NoError(t, err)

	cb, err := New(handler, triggerNetRatio, Fallback(fallbackRedirectPath))
	require.NoError(t, err)

	srv := httptest.NewServer(cb)
	t.Cleanup(srv.Close)

	cb.metrics = statsNetErrors(0.6)
	_, _, err = testutils.Get(srv.URL)
	require.NoError(t, err)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("no redirects")
		},
	}

	re, err := client.Get(srv.URL + "/somePath")
	require.Error(t, err)
	assert.Equal(t, http.StatusFound, re.StatusCode)
	assert.Equal(t, "http://localhost:6000/somePath", re.Header.Get("Location"))
}

func TestCircuitBreaker_redirect(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	fallbackRedirect, err := NewRedirectFallback(Redirect{URL: "http://localhost:5000"})
	require.NoError(t, err)

	cb, err := New(handler, triggerNetRatio, Fallback(fallbackRedirect))
	require.NoError(t, err)

	srv := httptest.NewServer(cb)
	t.Cleanup(srv.Close)

	cb.metrics = statsNetErrors(0.6)
	_, _, err = testutils.Get(srv.URL)
	require.NoError(t, err)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("no redirects")
		},
	}

	re, err := client.Get(srv.URL + "/somePath")
	require.Error(t, err)
	assert.Equal(t, http.StatusFound, re.StatusCode)
	assert.Equal(t, "http://localhost:5000", re.Header.Get("Location"))
}

func TestCircuitBreaker_triggerDuringRecovery(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	testutils.FreezeTime(t)

	cb, err := New(handler, triggerNetRatio, CheckPeriod(clock.Microsecond))
	require.NoError(t, err)

	srv := httptest.NewServer(cb)
	t.Cleanup(srv.Close)

	cb.metrics = statsNetErrors(0.6)
	_, _, err = testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, cbState(stateTripped), cb.state)

	// We should be in recovering state by now
	clock.Advance(10*clock.Second + clock.Millisecond)
	re, _, err := testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, re.StatusCode)
	assert.Equal(t, cbState(stateRecovering), cb.state)

	// We have matched error condition during recovery state and are going back to tripped state
	clock.Advance(5 * clock.Second)
	cb.metrics = statsNetErrors(0.6)
	allowed := 0
	for i := 0; i < 100; i++ {
		re, _, err = testutils.Get(srv.URL)
		if re.StatusCode == http.StatusOK && err == nil {
			allowed++
		}
	}
	assert.NotEqual(t, 0, allowed)
	assert.Equal(t, cbState(stateTripped), cb.state)
}

func TestCircuitBreaker_sideEffects(t *testing.T) {
	srv1Chan := make(chan *http.Request, 1)
	var srv1Body []byte
	srv1 := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		srv1Body = b
		_, _ = w.Write([]byte("srv1"))
		srv1Chan <- r
	})
	defer srv1.Close()

	srv2Chan := make(chan *http.Request, 1)
	srv2 := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("srv2"))
		err := r.ParseForm()
		require.NoError(t, err)
		srv2Chan <- r
	})
	defer srv2.Close()

	onTripped, err := NewWebhookSideEffect(
		Webhook{
			URL:     fmt.Sprintf("%s/post.json", srv1.URL),
			Method:  http.MethodPost,
			Headers: map[string][]string{"Content-Type": {"application/json"}},
			Body:    []byte(`{"Key": ["val1", "val2"]}`),
		})
	require.NoError(t, err)

	onStandby, err := NewWebhookSideEffect(
		Webhook{
			URL:    fmt.Sprintf("%s/post", srv2.URL),
			Method: http.MethodPost,
			Form:   map[string][]string{"key": {"val1", "val2"}},
		})
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	testutils.FreezeTime(t)

	cb, err := New(handler, triggerNetRatio, CheckPeriod(clock.Microsecond), OnTripped(onTripped), OnStandby(onStandby))
	require.NoError(t, err)

	srv := httptest.NewServer(cb)
	t.Cleanup(srv.Close)

	cb.metrics = statsNetErrors(0.6)

	_, _, err = testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, cbState(stateTripped), cb.state)

	select {
	case req := <-srv1Chan:
		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "/post.json", req.URL.Path)
		assert.JSONEq(t, `{"Key": ["val1", "val2"]}`, string(srv1Body))
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	case <-clock.After(clock.Second):
		t.Error("timeout waiting for side effect to kick off")
	}

	// Transition to recovering state
	clock.Advance(10*clock.Second + clock.Millisecond)
	cb.metrics = statsOK()
	_, _, err = testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, cbState(stateRecovering), cb.state)

	// Going back to standby
	clock.Advance(10*clock.Second + clock.Millisecond)
	_, _, err = testutils.Get(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, cbState(stateStandby), cb.state)

	select {
	case req := <-srv2Chan:
		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "/post", req.URL.Path)
		assert.Equal(t, url.Values{"key": []string{"val1", "val2"}}, req.Form)
	case <-clock.After(clock.Second):
		t.Error("timeout waiting for side effect to kick off")
	}
}

func statsOK() *memmetrics.RTMetrics {
	m, err := memmetrics.NewRTMetrics()
	if err != nil {
		panic(err)
	}
	return m
}

func statsNetErrors(threshold float64) *memmetrics.RTMetrics {
	m, err := memmetrics.NewRTMetrics()
	if err != nil {
		panic(err)
	}
	for i := 0; i < 100; i++ {
		if i < int(threshold*100) {
			m.Record(http.StatusGatewayTimeout, 0)
		} else {
			m.Record(http.StatusOK, 0)
		}
	}
	return m
}

func statsLatencyAtQuantile(_ float64, value time.Duration) *memmetrics.RTMetrics {
	m, err := memmetrics.NewRTMetrics()
	if err != nil {
		panic(err)
	}
	m.Record(http.StatusOK, value)
	return m
}

func statsResponseCodes(codes ...statusCode) *memmetrics.RTMetrics {
	m, err := memmetrics.NewRTMetrics()
	if err != nil {
		panic(err)
	}
	for _, c := range codes {
		for i := int64(0); i < c.Count; i++ {
			m.Record(c.Code, 0)
		}
	}
	return m
}

type statusCode struct {
	Code  int
	Count int64
}
