package memmetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
)

func TestNewRatioCounterInvalidParams(t *testing.T) {
	clock := testutils.GetClock()

	// Bad buckets count
	_, err := NewRatioCounter(0, time.Second, RatioClock(clock))
	require.Error(t, err)

	// Too precise resolution
	_, err = NewRatioCounter(10, time.Millisecond, RatioClock(clock))
	require.Error(t, err)
}

func TestNotReady(t *testing.T) {
	clock := testutils.GetClock()

	// No data
	fr, err := NewRatioCounter(10, time.Second, RatioClock(clock))
	require.NoError(t, err)
	assert.Equal(t, false, fr.IsReady())
	assert.Equal(t, 0.0, fr.Ratio())

	// Not enough data
	fr, err = NewRatioCounter(10, time.Second, RatioClock(clock))
	require.NoError(t, err)
	fr.CountA()
	assert.Equal(t, false, fr.IsReady())
}

func TestNoB(t *testing.T) {
	fr, err := NewRatioCounter(1, time.Second, RatioClock(testutils.GetClock()))
	require.NoError(t, err)
	fr.IncA(1)
	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 1.0, fr.Ratio())
}

func TestNoA(t *testing.T) {
	fr, err := NewRatioCounter(1, time.Second, RatioClock(testutils.GetClock()))
	require.NoError(t, err)
	fr.IncB(1)
	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 0.0, fr.Ratio())
}

// Make sure that data is properly calculated over several buckets
func TestMultipleBuckets(t *testing.T) {
	clock := testutils.GetClock()

	fr, err := NewRatioCounter(3, time.Second, RatioClock(clock))
	require.NoError(t, err)

	fr.IncB(1)
	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	fr.IncA(1)

	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	fr.IncA(1)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, float64(2)/float64(3), fr.Ratio())
}

// Make sure that data is properly calculated over several buckets
// When we overwrite old data when the window is rolling
func TestOverwriteBuckets(t *testing.T) {
	clock := testutils.GetClock()

	fr, err := NewRatioCounter(3, time.Second, RatioClock(clock))
	require.NoError(t, err)

	fr.IncB(1)

	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	fr.IncA(1)

	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	fr.IncA(1)

	// This time we should overwrite the old data points
	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	fr.IncA(1)
	fr.IncB(2)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, float64(3)/float64(5), fr.Ratio())
}

// Make sure we cleanup the data after periods of inactivity
// So it does not mess up the stats
func TestInactiveBuckets(t *testing.T) {
	clock := testutils.GetClock()

	fr, err := NewRatioCounter(3, time.Second, RatioClock(clock))
	require.NoError(t, err)

	fr.IncB(1)

	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	fr.IncA(1)

	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	fr.IncA(1)

	// This time we should overwrite the old data points with new data
	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	fr.IncA(1)
	fr.IncB(2)

	// Jump to the last bucket and change the data
	clock.CurrentTime = clock.CurrentTime.Add(time.Second * 2)
	fr.IncB(1)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, float64(1)/float64(4), fr.Ratio())
}

func TestLongPeriodsOfInactivity(t *testing.T) {
	clock := testutils.GetClock()

	fr, err := NewRatioCounter(2, time.Second, RatioClock(clock))
	require.NoError(t, err)

	fr.IncB(1)

	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	fr.IncA(1)

	assert.Equal(t, true, fr.IsReady())
	assert.Equal(t, 0.5, fr.Ratio())

	// This time we should overwrite all data points
	clock.CurrentTime = clock.CurrentTime.Add(100 * time.Second)
	fr.IncA(1)
	assert.Equal(t, 1.0, fr.Ratio())
}

func TestNewRatioCounterReset(t *testing.T) {
	fr, err := NewRatioCounter(1, time.Second, RatioClock(testutils.GetClock()))
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
