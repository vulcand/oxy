package memmetrics

import (
	"testing"

	"github.com/mailgun/holster/v4/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
)

func TestNewRatioCounterInvalidParams(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	// Bad buckets count
	_, err := NewRatioCounter(0, clock.Second)
	require.Error(t, err)

	// Too precise resolution
	_, err = NewRatioCounter(10, clock.Millisecond)
	require.Error(t, err)
}

func TestNotReady(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	// No data
	fr, err := NewRatioCounter(10, clock.Second)
	require.NoError(t, err)
	assert.Equal(t, false, fr.IsReady())
	assert.Equal(t, 0.0, fr.Ratio())

	// Not enough data
	fr, err = NewRatioCounter(10, clock.Second)
	require.NoError(t, err)
	fr.CountA()
	assert.Equal(t, false, fr.IsReady())
}

func TestNoB(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()
	fr, err := NewRatioCounter(1, clock.Second)
	require.NoError(t, err)
	fr.IncA(1)
	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 1.0, fr.Ratio())
}

func TestNoA(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	fr, err := NewRatioCounter(1, clock.Second)
	require.NoError(t, err)
	fr.IncB(1)
	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 0.0, fr.Ratio())
}

// Make sure that data is properly calculated over several buckets.
func TestMultipleBuckets(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	fr, err := NewRatioCounter(3, clock.Second)
	require.NoError(t, err)

	fr.IncB(1)
	clock.Advance(clock.Second)
	fr.IncA(1)

	clock.Advance(clock.Second)
	fr.IncA(1)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, float64(2)/float64(3), fr.Ratio())
}

// Make sure that data is properly calculated over several buckets
// When we overwrite old data when the window is rolling.
func TestOverwriteBuckets(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	fr, err := NewRatioCounter(3, clock.Second)
	require.NoError(t, err)

	fr.IncB(1)

	clock.Advance(clock.Second)
	fr.IncA(1)

	clock.Advance(clock.Second)
	fr.IncA(1)

	// This time we should overwrite the old data points
	clock.Advance(clock.Second)
	fr.IncA(1)
	fr.IncB(2)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, float64(3)/float64(5), fr.Ratio())
}

// Make sure we cleanup the data after periods of inactivity
// So it does not mess up the stats.
func TestInactiveBuckets(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	fr, err := NewRatioCounter(3, clock.Second)
	require.NoError(t, err)

	fr.IncB(1)

	clock.Advance(clock.Second)
	fr.IncA(1)

	clock.Advance(clock.Second)
	fr.IncA(1)

	// This time we should overwrite the old data points with new data
	clock.Advance(clock.Second)
	fr.IncA(1)
	fr.IncB(2)

	// Jump to the last bucket and change the data
	clock.Advance(clock.Second * 2)
	fr.IncB(1)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, float64(1)/float64(4), fr.Ratio())
}

func TestLongPeriodsOfInactivity(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	fr, err := NewRatioCounter(2, clock.Second)
	require.NoError(t, err)

	fr.IncB(1)

	clock.Advance(clock.Second)
	fr.IncA(1)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 0.5, fr.Ratio())

	// This time we should overwrite all data points
	clock.Advance(100 * clock.Second)
	fr.IncA(1)
	assert.Equal(t, 1.0, fr.Ratio())
}

func TestNewRatioCounterReset(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	fr, err := NewRatioCounter(1, clock.Second)
	require.NoError(t, err)

	fr.IncB(1)
	fr.IncA(1)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 0.5, fr.Ratio())

	// Reset the counter
	fr.Reset()
	assert.Equal(t, false, fr.IsReady())

	// Now add some stats
	fr.IncA(2)

	// We are game again!
	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 1.0, fr.Ratio())
}
