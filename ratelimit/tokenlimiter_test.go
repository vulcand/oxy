package ratelimit

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
	"github.com/vulcand/oxy/v2/testutils"
	"github.com/vulcand/oxy/v2/utils"
)

func TestRateSetAdd(t *testing.T) {
	rs := NewRateSet()

	// Invalid period
	err := rs.Add(0, 1, 1)
	require.Error(t, err)

	// Invalid Average
	err = rs.Add(clock.Second, 0, 1)
	require.Error(t, err)

	// Invalid Burst
	err = rs.Add(clock.Second, 1, 0)
	require.Error(t, err)

	err = rs.Add(clock.Second, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, "map[1s:rate(1/1s, burst=1)]", rs.String())
}

// We've hit the limit and were able to proceed on the next time run.
func TestHitLimit(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	rates := NewRateSet()
	err := rates.Add(clock.Second, 1, 1)
	require.NoError(t, err)

	testutils.FreezeTime(t)

	l, err := New(handler, headerLimit, rates)
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	t.Cleanup(srv.Close)

	re, _, err := testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	// Next request from the same source hits rate limit
	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, 429, re.StatusCode)

	// Second later, the request from this ip will succeed
	clock.Advance(clock.Second)

	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

// We've failed to extract client ip.
func TestFailure(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	rates := NewRateSet()
	err := rates.Add(clock.Second, 1, 1)
	require.NoError(t, err)

	testutils.FreezeTime(t)

	l, err := New(handler, faultyExtract, rates)
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	t.Cleanup(srv.Close)

	re, _, err := testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, re.StatusCode)
}

// Make sure rates from different ips are controlled separately.
func TestIsolation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	rates := NewRateSet()
	err := rates.Add(clock.Second, 1, 1)
	require.NoError(t, err)

	testutils.FreezeTime(t)

	l, err := New(handler, headerLimit, rates)
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	t.Cleanup(srv.Close)

	re, _, err := testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	// Next request from the same source hits rate limit
	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, 429, re.StatusCode)

	// The request from other source can proceed
	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "b"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

// Make sure that expiration works (Expiration is triggered after significant amount of time passes).
func TestExpiration(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	rates := NewRateSet()
	err := rates.Add(clock.Second, 1, 1)
	require.NoError(t, err)

	testutils.FreezeTime(t)

	l, err := New(handler, headerLimit, rates)
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	t.Cleanup(srv.Close)

	re, _, err := testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	// Next request from the same source hits rate limit
	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, 429, re.StatusCode)

	// 24 hours later, the request from this ip will succeed
	clock.Advance(24 * clock.Hour)

	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

// If rate limiting configuration is valid, then it is applied.
func TestExtractRates(t *testing.T) {
	// Given
	extractRates := func(*http.Request) (*RateSet, error) {
		rates := NewRateSet()

		err := rates.Add(clock.Second, 2, 2)
		if err != nil {
			return nil, err
		}

		err = rates.Add(60*clock.Second, 10, 10)
		if err != nil {
			return nil, err
		}

		return rates, nil
	}

	rates := NewRateSet()
	err := rates.Add(clock.Second, 1, 1)
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	testutils.FreezeTime(t)

	tl, err := New(handler, headerLimit, rates, ExtractRates(RateExtractorFunc(extractRates)))
	require.NoError(t, err)

	srv := httptest.NewServer(tl)
	t.Cleanup(srv.Close)

	// When/Then: The configured rate is applied, which 2 req/second
	re, _, err := testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, 429, re.StatusCode)

	clock.Advance(clock.Second)

	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

// If configMapper returns error, then the default rate is applied.
func TestBadRateExtractor(t *testing.T) {
	// Given
	extractor := func(*http.Request) (*RateSet, error) {
		return nil, errors.New("boom")
	}

	rates := NewRateSet()
	err := rates.Add(clock.Second, 1, 1)
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	testutils.FreezeTime(t)

	l, err := New(handler, headerLimit, rates, ExtractRates(RateExtractorFunc(extractor)))
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	t.Cleanup(srv.Close)

	// When/Then: The default rate is applied, which 1 req/second
	re, _, err := testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, 429, re.StatusCode)

	clock.Advance(clock.Second)

	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

// If configMapper returns empty rates, then the default rate is applied.
func TestExtractorEmpty(t *testing.T) {
	// Given
	extractor := func(*http.Request) (*RateSet, error) {
		return NewRateSet(), nil
	}

	rates := NewRateSet()
	err := rates.Add(clock.Second, 1, 1)
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	testutils.FreezeTime(t)

	l, err := New(handler, headerLimit, rates, ExtractRates(RateExtractorFunc(extractor)))
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	t.Cleanup(srv.Close)

	// When/Then: The default rate is applied, which 1 req/second
	re, _, err := testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, 429, re.StatusCode)

	clock.Advance(clock.Second)

	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)
}

func TestInvalidParams(t *testing.T) {
	// Rates are missing
	rs := NewRateSet()
	err := rs.Add(clock.Second, 1, 1)
	require.NoError(t, err)

	// Empty
	_, err = New(nil, nil, rs)
	require.Error(t, err)

	// Rates are empty
	_, err = New(nil, nil, NewRateSet())
	require.Error(t, err)

	// Bad capacity
	_, err = New(nil, headerLimit, rs, Capacity(-1))
	require.Error(t, err)
}

// We've hit the limit and were able to proceed on the next time run.
func TestOptions(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	rates := NewRateSet()
	err := rates.Add(clock.Second, 1, 1)
	require.NoError(t, err)

	errHandler := utils.ErrorHandlerFunc(func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(http.StatusText(http.StatusTeapot)))
	})

	testutils.FreezeTime(t)

	l, err := New(handler, headerLimit, rates, ErrorHandler(errHandler))
	require.NoError(t, err)

	srv := httptest.NewServer(l)
	t.Cleanup(srv.Close)

	re, _, err := testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, re.StatusCode)

	re, _, err = testutils.Get(srv.URL, testutils.Header("Source", "a"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusTeapot, re.StatusCode)
}

func headerLimiter(req *http.Request) (string, int64, error) {
	return req.Header.Get("Source"), 1, nil
}

func faultyExtractor(_ *http.Request) (string, int64, error) {
	return "", -1, errors.New("oops")
}

var headerLimit = utils.ExtractorFunc(headerLimiter)

var faultyExtract = utils.ExtractorFunc(faultyExtractor)
