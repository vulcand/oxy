package ratelimit

import (
	"testing"
	"time"

	"github.com/mailgun/timetools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
)

func TestConsumeSingleToken(t *testing.T) {
	clock := testutils.GetClock()

	tb := newTokenBucket(&rate{period: time.Second, average: 1, burst: 1}, clock)

	// First request passes
	delay, err := tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	// Next request does not pass the same second
	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Second, delay)

	// Second later, the request passes
	clock.Sleep(time.Second)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	// Five seconds later, still only one request is allowed
	// because maxBurst is 1
	clock.Sleep(5 * time.Second)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	// The next one is forbidden
	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Second, delay)
}

func TestFastConsumption(t *testing.T) {
	clock := testutils.GetClock()

	tb := newTokenBucket(&rate{period: time.Second, average: 1, burst: 1}, clock)

	// First request passes
	delay, err := tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	// Try 200 ms later
	clock.Sleep(time.Millisecond * 200)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Second, delay)

	// Try 700 ms later
	clock.Sleep(time.Millisecond * 700)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Second, delay)

	// Try 100 ms later, success!
	clock.Sleep(time.Millisecond * 100)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)
}

func TestConsumeMultipleTokens(t *testing.T) {
	tb := newTokenBucket(&rate{period: time.Second, average: 3, burst: 5}, testutils.GetClock())

	delay, err := tb.consume(3)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	delay, err = tb.consume(2)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.NotEqual(t, time.Duration(0), delay)
}

func TestDelayIsCorrect(t *testing.T) {
	clock := testutils.GetClock()

	tb := newTokenBucket(&rate{period: time.Second, average: 3, burst: 5}, clock)

	// Exhaust initial capacity
	delay, err := tb.consume(5)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	delay, err = tb.consume(3)
	require.NoError(t, err)
	assert.NotEqual(t, time.Duration(0), delay)

	// Now wait provided delay and make sure we can consume now
	clock.Sleep(delay)

	delay, err = tb.consume(3)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)
}

// Make sure requests that exceed burst size are not allowed
func TestExceedsBurst(t *testing.T) {
	tb := newTokenBucket(&rate{period: time.Second, average: 1, burst: 10}, testutils.GetClock())

	_, err := tb.consume(11)
	require.Error(t, err)
}

func TestConsumeBurst(t *testing.T) {
	tb := newTokenBucket(&rate{period: time.Second, average: 2, burst: 5}, testutils.GetClock())

	// In two seconds we would have 5 tokens
	testutils.GetClock().Sleep(2 * time.Second)

	// Lets consume 5 at once
	delay, err := tb.consume(5)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)
}

func TestConsumeEstimate(t *testing.T) {
	tb := newTokenBucket(&rate{period: time.Second, average: 2, burst: 4}, testutils.GetClock())

	// Consume all burst at once
	delay, err := tb.consume(4)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	// Now try to consume it and face delay
	delay, err = tb.consume(4)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(2)*time.Second, delay)
}

// If a rate with different period is passed to the `update` method, then an
// error is returned but the state of the bucket remains valid and unchanged.
func TestUpdateInvalidPeriod(t *testing.T) {
	clock := testutils.GetClock()

	// Given
	tb := newTokenBucket(&rate{period: time.Second, average: 10, burst: 20}, clock)
	_, err := tb.consume(15) // 5 tokens available
	require.NoError(t, err)

	// When
	err = tb.update(&rate{period: time.Second + 1, average: 30, burst: 40}) // still 5 tokens available
	require.Error(t, err)

	// Then

	// ...check that rate did not change
	clock.Sleep(500 * time.Millisecond)

	delay, err := tb.consume(11)
	require.NoError(t, err)
	assert.Equal(t, 100*time.Millisecond, delay)

	delay, err = tb.consume(10)
	require.NoError(t, err)
	// 0 available
	assert.Equal(t, time.Duration(0), delay)

	// ...check that burst did not change
	clock.Sleep(40 * time.Second)
	_, err = tb.consume(21)
	require.Error(t, err)

	delay, err = tb.consume(20)
	require.NoError(t, err)
	// 0 available
	assert.Equal(t, time.Duration(0), delay)
}

// If the capacity of the bucket is increased by the update then it takes some
// time to fill the bucket with tokens up to the new capacity.
func TestUpdateBurstIncreased(t *testing.T) {
	clock := testutils.GetClock()

	// Given
	tb := newTokenBucket(&rate{period: time.Second, average: 10, burst: 20}, clock)
	_, err := tb.consume(15) // 5 tokens available
	require.NoError(t, err)

	// When
	err = tb.update(&rate{period: time.Second, average: 10, burst: 50}) // still 5 tokens available
	require.NoError(t, err)

	// Then
	delay, err := tb.consume(50)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(time.Second/10*45), delay)
}

// If the capacity of the bucket is increased by the update then it takes some
// time to fill the bucket with tokens up to the new capacity.
func TestUpdateBurstDecreased(t *testing.T) {
	clock := testutils.GetClock()

	// Given
	tb := newTokenBucket(&rate{period: time.Second, average: 10, burst: 50}, clock)
	_, err := tb.consume(15) // 35 tokens available
	require.NoError(t, err)

	// When
	err = tb.update(&rate{period: time.Second, average: 10, burst: 20}) // the number of available tokens reduced to 20.
	require.NoError(t, err)

	// Then
	delay, err := tb.consume(21)
	require.Error(t, err)
	assert.Equal(t, time.Duration(-1), delay)
}

// If rate is updated then it affects the bucket refill speed.
func TestUpdateRateChanged(t *testing.T) {
	clock := testutils.GetClock()

	// Given
	tb := newTokenBucket(&rate{period: time.Second, average: 10, burst: 20}, clock)
	_, err := tb.consume(15) // 5 tokens available
	require.NoError(t, err)

	// When
	err = tb.update(&rate{period: time.Second, average: 20, burst: 20}) // still 5 tokens available
	require.NoError(t, err)

	// Then
	delay, err := tb.consume(20)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(time.Second/20*15), delay)
}

// Only the most recent consumption is reverted by `Rollback`.
func TestRollback(t *testing.T) {
	clock := testutils.GetClock()

	// Given
	tb := newTokenBucket(&rate{period: time.Second, average: 10, burst: 20}, clock)
	_, err := tb.consume(8) // 12 tokens available
	require.NoError(t, err)
	_, err = tb.consume(7) // 5 tokens available
	require.NoError(t, err)

	// When
	tb.rollback() // 12 tokens available

	// Then
	delay, err := tb.consume(12)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, 100*time.Millisecond, delay)
}

// It is safe to call `Rollback` several times. The second and all subsequent
// calls just do nothing.
func TestRollbackSeveralTimes(t *testing.T) {
	// Given
	tb := newTokenBucket(&rate{period: time.Second, average: 10, burst: 20}, testutils.GetClock())
	_, err := tb.consume(8) // 12 tokens available
	require.NoError(t, err)
	tb.rollback() // 20 tokens available

	// When
	tb.rollback() // still 20 tokens available
	tb.rollback() // still 20 tokens available
	tb.rollback() // still 20 tokens available

	// Then: all 20 tokens can be consumed
	delay, err := tb.consume(20)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, 100*time.Millisecond, delay)
}

// If previous consumption returned a delay due to an attempt to consume more
// tokens then there are available, then `Rollback` has no effect.
func TestRollbackAfterAvailableExceeded(t *testing.T) {
	// Given
	tb := newTokenBucket(&rate{period: time.Second, average: 10, burst: 20}, testutils.GetClock())
	_, err := tb.consume(8) // 12 tokens available
	require.NoError(t, err)
	delay, err := tb.consume(15) // still 12 tokens available
	require.NoError(t, err)
	assert.Equal(t, 300*time.Millisecond, delay)

	// When
	tb.rollback() // Previous operation consumed 0 tokens, so rollback has no effect.

	// Then
	delay, err = tb.consume(12)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, 100*time.Millisecond, delay)
}

// If previous consumption returned a error due to an attempt to consume more
// tokens then the bucket's burst size, then `Rollback` has no effect.
func TestRollbackAfterError(t *testing.T) {
	clock := testutils.GetClock()

	// Given
	tb := newTokenBucket(&rate{period: time.Second, average: 10, burst: 20}, clock)
	_, err := tb.consume(8) // 12 tokens available
	require.NoError(t, err)
	delay, err := tb.consume(21) // still 12 tokens available
	require.Error(t, err)
	assert.Equal(t, time.Duration(-1), delay)

	// When
	tb.rollback() // Previous operation consumed 0 tokens, so rollback has no effect.

	// Then
	delay, err = tb.consume(12)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, 100*time.Millisecond, delay)
}

func TestDivisionByZeroOnPeriod(t *testing.T) {
	clock := &timetools.RealTime{}

	var emptyPeriod int64
	tb := newTokenBucket(&rate{period: time.Duration(emptyPeriod), average: 2, burst: 2}, clock)

	_, err := tb.consume(1)
	assert.NoError(t, err)

	err = tb.update(&rate{period: time.Nanosecond, average: 1, burst: 1})
	assert.NoError(t, err)
}
