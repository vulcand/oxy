package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
	"github.com/vulcand/oxy/v2/testutils"
)

func TestConsumeSingleToken(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	tb := newTokenBucket(&rate{period: clock.Second, average: 1, burst: 1})

	// First request passes
	delay, err := tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	// Next request does not pass the same second
	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, clock.Second, delay)

	// Second later, the request passes
	clock.Advance(clock.Second)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	// Five seconds later, still only one request is allowed
	// because maxBurst is 1
	clock.Advance(5 * clock.Second)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	// The next one is forbidden
	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, clock.Second, delay)
}

func TestFastConsumption(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	tb := newTokenBucket(&rate{period: clock.Second, average: 1, burst: 1})

	// First request passes
	delay, err := tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	// Try 200 ms later
	clock.Advance(clock.Millisecond * 200)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, clock.Second, delay)

	// Try 700 ms later
	clock.Advance(clock.Millisecond * 700)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, clock.Second, delay)

	// Try 100 ms later, success!
	clock.Advance(clock.Millisecond * 100)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)
}

func TestConsumeMultipleTokens(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	tb := newTokenBucket(&rate{period: clock.Second, average: 3, burst: 5})

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
	done := testutils.FreezeTime()
	defer done()

	tb := newTokenBucket(&rate{period: clock.Second, average: 3, burst: 5})

	// Exhaust initial capacity
	delay, err := tb.consume(5)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	delay, err = tb.consume(3)
	require.NoError(t, err)
	assert.NotEqual(t, time.Duration(0), delay)

	// Now wait provided delay and make sure we can consume now
	clock.Advance(delay)

	delay, err = tb.consume(3)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)
}

// Make sure requests that exceed burst size are not allowed.
func TestExceedsBurst(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	tb := newTokenBucket(&rate{period: clock.Second, average: 1, burst: 10})

	_, err := tb.consume(11)
	require.Error(t, err)
}

func TestConsumeBurst(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	tb := newTokenBucket(&rate{period: clock.Second, average: 2, burst: 5})

	// In two seconds we would have 5 tokens
	clock.Advance(2 * clock.Second)

	// Lets consume 5 at once
	delay, err := tb.consume(5)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)
}

func TestConsumeEstimate(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	tb := newTokenBucket(&rate{period: clock.Second, average: 2, burst: 4})

	// Consume all burst at once
	delay, err := tb.consume(4)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	// Now try to consume it and face delay
	delay, err = tb.consume(4)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(2)*clock.Second, delay)
}

// If a rate with different period is passed to the `update` method, then an
// error is returned but the state of the bucket remains valid and unchanged.
func TestUpdateInvalidPeriod(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	// Given
	tb := newTokenBucket(&rate{period: clock.Second, average: 10, burst: 20})
	_, err := tb.consume(15) // 5 tokens available
	require.NoError(t, err)

	// When
	err = tb.update(&rate{period: clock.Second + 1, average: 30, burst: 40}) // still 5 tokens available
	require.Error(t, err)

	// Then

	// ...check that rate did not change
	clock.Advance(500 * clock.Millisecond)

	delay, err := tb.consume(11)
	require.NoError(t, err)
	assert.Equal(t, 100*clock.Millisecond, delay)

	delay, err = tb.consume(10)
	require.NoError(t, err)
	// 0 available
	assert.Equal(t, time.Duration(0), delay)

	// ...check that burst did not change
	clock.Advance(40 * clock.Second)
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
	done := testutils.FreezeTime()
	defer done()

	// Given
	tb := newTokenBucket(&rate{period: clock.Second, average: 10, burst: 20})
	_, err := tb.consume(15) // 5 tokens available
	require.NoError(t, err)

	// When
	err = tb.update(&rate{period: clock.Second, average: 10, burst: 50}) // still 5 tokens available
	require.NoError(t, err)

	// Then
	delay, err := tb.consume(50)
	require.NoError(t, err)
	assert.Equal(t, clock.Second/10*45, delay)
}

// If the capacity of the bucket is increased by the update then it takes some
// time to fill the bucket with tokens up to the new capacity.
func TestUpdateBurstDecreased(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	// Given
	tb := newTokenBucket(&rate{period: clock.Second, average: 10, burst: 50})
	_, err := tb.consume(15) // 35 tokens available
	require.NoError(t, err)

	// When
	err = tb.update(&rate{period: clock.Second, average: 10, burst: 20}) // the number of available tokens reduced to 20.
	require.NoError(t, err)

	// Then
	delay, err := tb.consume(21)
	require.Error(t, err)
	assert.Equal(t, time.Duration(-1), delay)
}

// If rate is updated then it affects the bucket refill speed.
func TestUpdateRateChanged(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	// Given
	tb := newTokenBucket(&rate{period: clock.Second, average: 10, burst: 20})
	_, err := tb.consume(15) // 5 tokens available
	require.NoError(t, err)

	// When
	err = tb.update(&rate{period: clock.Second, average: 20, burst: 20}) // still 5 tokens available
	require.NoError(t, err)

	// Then
	delay, err := tb.consume(20)
	require.NoError(t, err)
	assert.Equal(t, clock.Second/20*15, delay)
}

// Only the most recent consumption is reverted by `Rollback`.
func TestRollback(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	// Given
	tb := newTokenBucket(&rate{period: clock.Second, average: 10, burst: 20})
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
	assert.Equal(t, 100*clock.Millisecond, delay)
}

// It is safe to call `Rollback` several times. The second and all subsequent
// calls just do nothing.
func TestRollbackSeveralTimes(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	// Given
	tb := newTokenBucket(&rate{period: clock.Second, average: 10, burst: 20})
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
	assert.Equal(t, 100*clock.Millisecond, delay)
}

// If previous consumption returned a delay due to an attempt to consume more
// tokens then there are available, then `Rollback` has no effect.
func TestRollbackAfterAvailableExceeded(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	// Given
	tb := newTokenBucket(&rate{period: clock.Second, average: 10, burst: 20})
	_, err := tb.consume(8) // 12 tokens available
	require.NoError(t, err)
	delay, err := tb.consume(15) // still 12 tokens available
	require.NoError(t, err)
	assert.Equal(t, 300*clock.Millisecond, delay)

	// When
	tb.rollback() // Previous operation consumed 0 tokens, so rollback has no effect.

	// Then
	delay, err = tb.consume(12)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)

	delay, err = tb.consume(1)
	require.NoError(t, err)
	assert.Equal(t, 100*clock.Millisecond, delay)
}

// If previous consumption returned a error due to an attempt to consume more
// tokens then the bucket's burst size, then `Rollback` has no effect.
func TestRollbackAfterError(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	// Given
	tb := newTokenBucket(&rate{period: clock.Second, average: 10, burst: 20})
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
	assert.Equal(t, 100*clock.Millisecond, delay)
}

func TestDivisionByZeroOnPeriod(t *testing.T) {
	var emptyPeriod int64
	tb := newTokenBucket(&rate{period: time.Duration(emptyPeriod), average: 2, burst: 2})

	_, err := tb.consume(1)
	assert.NoError(t, err)

	err = tb.update(&rate{period: clock.Nanosecond, average: 1, burst: 1})
	assert.NoError(t, err)
}
