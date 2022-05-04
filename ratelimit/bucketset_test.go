package ratelimit

import (
	"testing"

	"github.com/mailgun/holster/v4/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
)

// A value returned by `MaxPeriod` corresponds to the longest bucket time period.
func TestLongestPeriod(t *testing.T) {
	// Given
	rates := NewRateSet()
	require.NoError(t, rates.Add(1*clock.Second, 10, 20))
	require.NoError(t, rates.Add(7*clock.Second, 10, 20))
	require.NoError(t, rates.Add(5*clock.Second, 11, 21))

	done := testutils.FreezeTime()
	defer done()

	// When
	tbs := NewTokenBucketSet(rates)

	// Then
	assert.Equal(t, 7*clock.Second, tbs.maxPeriod)
}

// Successful token consumption updates state of all buckets in the set.
func TestConsume(t *testing.T) {
	// Given
	rates := NewRateSet()
	require.NoError(t, rates.Add(1*clock.Second, 10, 20))
	require.NoError(t, rates.Add(10*clock.Second, 20, 50))

	done := testutils.FreezeTime()
	defer done()

	tbs := NewTokenBucketSet(rates)

	// When
	delay, err := tbs.Consume(15)
	require.NoError(t, err)

	// Then
	assert.Equal(t, clock.Duration(0), delay)
	assert.Equal(t, "{1s: 5}, {10s: 35}", tbs.debugState())
}

// As time goes by all set buckets are refilled with appropriate rates.
func TestConsumeRefill(t *testing.T) {
	// Given
	rates := NewRateSet()
	require.NoError(t, rates.Add(10*clock.Second, 10, 20))
	require.NoError(t, rates.Add(100*clock.Second, 20, 50))

	done := testutils.FreezeTime()
	defer done()

	tbs := NewTokenBucketSet(rates)

	_, err := tbs.Consume(15)
	require.NoError(t, err)
	assert.Equal(t, "{10s: 5}, {1m40s: 35}", tbs.debugState())

	// When
	clock.Advance(10 * clock.Second)

	delay, err := tbs.Consume(0) // Consumes nothing but forces an internal state update.
	require.NoError(t, err)

	// Then
	assert.Equal(t, clock.Duration(0), delay)
	assert.Equal(t, "{10s: 15}, {1m40s: 37}", tbs.debugState())
}

// If the first bucket in the set has no enough tokens to allow desired
// consumption then an appropriate delay is returned.
func TestConsumeLimitedBy1st(t *testing.T) {
	// Given
	rates := NewRateSet()
	require.NoError(t, rates.Add(10*clock.Second, 10, 10))
	require.NoError(t, rates.Add(100*clock.Second, 20, 20))

	done := testutils.FreezeTime()
	defer done()

	tbs := NewTokenBucketSet(rates)

	_, err := tbs.Consume(5)
	require.NoError(t, err)
	assert.Equal(t, "{10s: 5}, {1m40s: 15}", tbs.debugState())

	// When
	delay, err := tbs.Consume(10)
	require.NoError(t, err)

	// Then
	assert.Equal(t, 5*clock.Second, delay)
	assert.Equal(t, "{10s: 5}, {1m40s: 15}", tbs.debugState())
}

// If the second bucket in the set has no enough tokens to allow desired
// consumption then an appropriate delay is returned.
func TestConsumeLimitedBy2st(t *testing.T) {
	// Given
	rates := NewRateSet()
	require.NoError(t, rates.Add(10*clock.Second, 10, 10))
	require.NoError(t, rates.Add(100*clock.Second, 20, 20))

	done := testutils.FreezeTime()
	defer done()

	tbs := NewTokenBucketSet(rates)

	_, err := tbs.Consume(10)
	require.NoError(t, err)

	clock.Advance(10 * clock.Second)

	_, err = tbs.Consume(10)
	require.NoError(t, err)

	clock.Advance(5 * clock.Second)

	_, err = tbs.Consume(0)
	require.NoError(t, err)
	assert.Equal(t, "{10s: 5}, {1m40s: 3}", tbs.debugState())

	// When
	delay, err := tbs.Consume(10)
	require.NoError(t, err)

	// Then
	assert.Equal(t, 7*(5*clock.Second), delay)
	assert.Equal(t, "{10s: 5}, {1m40s: 3}", tbs.debugState())
}

// An attempt to consume more tokens then the smallest bucket capacity results
// in error.
func TestConsumeMoreThenBurst(t *testing.T) {
	// Given
	rates := NewRateSet()
	require.NoError(t, rates.Add(1*clock.Second, 10, 20))
	require.NoError(t, rates.Add(10*clock.Second, 50, 100))

	done := testutils.FreezeTime()
	defer done()

	tbs := NewTokenBucketSet(rates)

	_, err := tbs.Consume(5)
	require.NoError(t, err)
	assert.Equal(t, "{1s: 15}, {10s: 95}", tbs.debugState())

	// When
	_, err = tbs.Consume(21)
	require.Error(t, err)

	// Then
	assert.Equal(t, "{1s: 15}, {10s: 95}", tbs.debugState())
}

// Update operation can add buckets.
func TestUpdateMore(t *testing.T) {
	// Given
	rates := NewRateSet()
	require.NoError(t, rates.Add(1*clock.Second, 10, 20))
	require.NoError(t, rates.Add(10*clock.Second, 20, 50))
	require.NoError(t, rates.Add(20*clock.Second, 45, 90))

	done := testutils.FreezeTime()
	defer done()

	tbs := NewTokenBucketSet(rates)

	_, err := tbs.Consume(5)
	require.NoError(t, err)
	assert.Equal(t, "{1s: 15}, {10s: 45}, {20s: 85}", tbs.debugState())

	rates = NewRateSet()
	require.NoError(t, rates.Add(10*clock.Second, 30, 40))
	require.NoError(t, rates.Add(11*clock.Second, 30, 40))
	require.NoError(t, rates.Add(12*clock.Second, 30, 40))
	require.NoError(t, rates.Add(13*clock.Second, 30, 40))

	// When
	tbs.Update(rates)

	// Then
	assert.Equal(t, "{10s: 40}, {11s: 40}, {12s: 40}, {13s: 40}", tbs.debugState())
	assert.Equal(t, 13*clock.Second, tbs.maxPeriod)
}

// Update operation can remove buckets.
func TestUpdateLess(t *testing.T) {
	// Given
	rates := NewRateSet()
	require.NoError(t, rates.Add(1*clock.Second, 10, 20))
	require.NoError(t, rates.Add(10*clock.Second, 20, 50))
	require.NoError(t, rates.Add(20*clock.Second, 45, 90))
	require.NoError(t, rates.Add(30*clock.Second, 50, 100))

	done := testutils.FreezeTime()
	defer done()

	tbs := NewTokenBucketSet(rates)

	_, err := tbs.Consume(5)
	require.NoError(t, err)
	assert.Equal(t, "{1s: 15}, {10s: 45}, {20s: 85}, {30s: 95}", tbs.debugState())

	rates = NewRateSet()
	require.NoError(t, rates.Add(10*clock.Second, 25, 20))
	require.NoError(t, rates.Add(20*clock.Second, 30, 21))

	// When
	tbs.Update(rates)

	// Then
	assert.Equal(t, "{10s: 20}, {20s: 21}", tbs.debugState())
	assert.Equal(t, 20*clock.Second, tbs.maxPeriod)
}

// Update operation can remove buckets.
func TestUpdateAllDifferent(t *testing.T) {
	// Given
	rates := NewRateSet()
	require.NoError(t, rates.Add(10*clock.Second, 20, 50))
	require.NoError(t, rates.Add(30*clock.Second, 50, 100))

	done := testutils.FreezeTime()
	defer done()

	tbs := NewTokenBucketSet(rates)

	_, err := tbs.Consume(5)
	require.NoError(t, err)
	assert.Equal(t, "{10s: 45}, {30s: 95}", tbs.debugState())

	rates = NewRateSet()
	require.NoError(t, rates.Add(1*clock.Second, 10, 40))
	require.NoError(t, rates.Add(60*clock.Second, 100, 150))

	// When
	tbs.Update(rates)

	// Then
	assert.Equal(t, "{1s: 40}, {1m0s: 150}", tbs.debugState())
	assert.Equal(t, 60*clock.Second, tbs.maxPeriod)
}
