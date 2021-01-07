package memmetrics

import (
	"testing"
	"time"

	"github.com/mailgun/holster/v3/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRatioCounterInvalidParams(t *testing.T) {
	defer clock.Freeze(time.Now()).Unfreeze()

	// Bad buckets count
	_, err := NewRatioCounter(0, time.Second)
	require.Error(t, err)

	// Too precise resolution
	_, err = NewRatioCounter(10, time.Millisecond)
	require.Error(t, err)
}

func TestNotReady(t *testing.T) {
	defer clock.Freeze(time.Now()).Unfreeze()

	// No data
	fr, err := NewRatioCounter(10, time.Second)
	require.NoError(t, err)
	assert.Equal(t, false, fr.IsReady())
	assert.Equal(t, 0.0, fr.Ratio())

	// Not enough data
	fr, err = NewRatioCounter(10, time.Second)
	require.NoError(t, err)
	fr.CountA()
	assert.Equal(t, false, fr.IsReady())
}

func TestNoB(t *testing.T) {
	defer clock.Freeze(time.Now()).Unfreeze()
	fr, err := NewRatioCounter(1, time.Second)
	require.NoError(t, err)
	fr.IncA(1)
	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 1.0, fr.Ratio())
}

func TestNoA(t *testing.T) {
	defer clock.Freeze(time.Now()).Unfreeze()
	fr, err := NewRatioCounter(1, time.Second)
	require.NoError(t, err)
	fr.IncB(1)
	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 0.0, fr.Ratio())
}

// Make sure that data is properly calculated over several buckets
func TestMultipleBuckets(t *testing.T) {
	defer clock.Freeze(time.Now()).Unfreeze()

	fr, err := NewRatioCounter(3, time.Second)
	require.NoError(t, err)

	fr.IncB(1)
	clock.Advance(time.Second)
	fr.IncA(1)

	clock.Advance(time.Second)
	fr.IncA(1)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, float64(2)/float64(3), fr.Ratio())
}

// Make sure that data is properly calculated over several buckets
// When we overwrite old data when the window is rolling
func TestOverwriteBuckets(t *testing.T) {
	defer clock.Freeze(time.Now()).Unfreeze()

	fr, err := NewRatioCounter(3, time.Second)
	require.NoError(t, err)

	fr.IncB(1)

	clock.Advance(time.Second)
	fr.IncA(1)

	clock.Advance(time.Second)
	fr.IncA(1)

	// This time we should overwrite the old data points
	clock.Advance(time.Second)
	fr.IncA(1)
	fr.IncB(2)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, float64(3)/float64(5), fr.Ratio())
}

// Make sure we cleanup the data after periods of inactivity
// So it does not mess up the stats
func TestInactiveBuckets(t *testing.T) {
	defer clock.Freeze(time.Now()).Unfreeze()

	fr, err := NewRatioCounter(3, time.Second)
	require.NoError(t, err)

	fr.IncB(1)

	clock.Advance(time.Second)
	fr.IncA(1)

	clock.Advance(time.Second)
	fr.IncA(1)

	// This time we should overwrite the old data points with new data
	clock.Advance(time.Second)
	fr.IncA(1)
	fr.IncB(2)

	// Jump to the last bucket and change the data
	clock.Advance(time.Second * 2)
	fr.IncB(1)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, float64(1)/float64(4), fr.Ratio())
}

func TestLongPeriodsOfInactivity(t *testing.T) {
	defer clock.Freeze(time.Now()).Unfreeze()

	fr, err := NewRatioCounter(2, time.Second)
	require.NoError(t, err)

	fr.IncB(1)

	clock.Advance(time.Second)
	fr.IncA(1)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 0.5, fr.Ratio())

	// This time we should overwrite all data points
	clock.Advance(100 * time.Second)
	fr.IncA(1)
	assert.Equal(t, 1.0, fr.Ratio())
}

func TestNewRatioCounterReset(t *testing.T) {
	defer clock.Freeze(time.Now()).Unfreeze()
	fr, err := NewRatioCounter(1, time.Second)
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
