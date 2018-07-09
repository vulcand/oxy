package memmetrics

import (
	"testing"
	"time"

	"github.com/codahale/hdrhistogram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
)

func TestMerge(t *testing.T) {
	a, err := NewHDRHistogram(1, 3600000, 2)
	require.NoError(t, err)

	err = a.RecordValues(1, 2)
	require.NoError(t, err)

	b, err := NewHDRHistogram(1, 3600000, 2)
	require.NoError(t, err)

	require.NoError(t, b.RecordValues(2, 1))
	require.NoError(t, a.Merge(b))

	assert.EqualValues(t, 1, a.ValueAtQuantile(50))
	assert.EqualValues(t, 2, a.ValueAtQuantile(100))
}

func TestInvalidParams(t *testing.T) {
	_, err := NewHDRHistogram(1, 3600000, 0)
	require.Error(t, err)
}

func TestMergeNil(t *testing.T) {
	a, err := NewHDRHistogram(1, 3600000, 1)
	require.NoError(t, err)

	require.Error(t, a.Merge(nil))
}

func TestRotation(t *testing.T) {
	clock := testutils.GetClock()

	h, err := NewRollingHDRHistogram(
		1,           // min value
		3600000,     // max value
		3,           // significant figures
		time.Second, // 1 second is a rolling period
		2,           // 2 histograms in a window
		RollingClock(clock))

	require.NoError(t, err)
	require.NotNil(t, h)

	err = h.RecordValues(5, 1)
	require.NoError(t, err)

	m, err := h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 5, m.ValueAtQuantile(100))

	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	require.NoError(t, h.RecordValues(2, 1))
	require.NoError(t, h.RecordValues(1, 1))

	m, err = h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 5, m.ValueAtQuantile(100))

	// rotate, this means that the old value would evaporate
	clock.CurrentTime = clock.CurrentTime.Add(time.Second)

	require.NoError(t, h.RecordValues(1, 1))

	m, err = h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 2, m.ValueAtQuantile(100))
}

func TestReset(t *testing.T) {
	clock := testutils.GetClock()

	h, err := NewRollingHDRHistogram(
		1,           // min value
		3600000,     // max value
		3,           // significant figures
		time.Second, // 1 second is a rolling period
		2,           // 2 histograms in a window
		RollingClock(clock))

	require.NoError(t, err)
	require.NotNil(t, h)

	require.NoError(t, h.RecordValues(5, 1))

	m, err := h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 5, m.ValueAtQuantile(100))

	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	require.NoError(t, h.RecordValues(2, 1))
	require.NoError(t, h.RecordValues(1, 1))

	m, err = h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 5, m.ValueAtQuantile(100))

	h.Reset()

	require.NoError(t, h.RecordValues(5, 1))

	m, err = h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 5, m.ValueAtQuantile(100))

	clock.CurrentTime = clock.CurrentTime.Add(time.Second)
	require.NoError(t, h.RecordValues(2, 1))
	require.NoError(t, h.RecordValues(1, 1))

	m, err = h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 5, m.ValueAtQuantile(100))

}

func TestHDRHistogramExportReturnsNewCopy(t *testing.T) {
	// Create HDRHistogram instance
	a := HDRHistogram{
		low:     1,
		high:    2,
		sigfigs: 3,
		h:       hdrhistogram.New(0, 1, 2),
	}

	// Get a copy and modify the original
	b := a.Export()
	a.low = 11
	a.high = 12
	a.sigfigs = 4
	a.h = nil

	// Assert the copy has not been modified
	assert.EqualValues(t, 1, b.low)
	assert.EqualValues(t, 2, b.high)
	assert.Equal(t, 3, b.sigfigs)
	require.NotNil(t, b.h)
}

func TestRollingHDRHistogramExportReturnsNewCopy(t *testing.T) {
	origTime := time.Now()

	a := RollingHDRHistogram{
		idx:         1,
		lastRoll:    origTime,
		period:      2 * time.Second,
		bucketCount: 3,
		low:         4,
		high:        5,
		sigfigs:     1,
		buckets:     []*HDRHistogram{},
		clock:       testutils.GetClock(),
	}

	b := a.Export()
	a.idx = 11
	a.lastRoll = time.Now().Add(1 * time.Minute)
	a.period = 12 * time.Second
	a.bucketCount = 13
	a.low = 14
	a.high = 15
	a.sigfigs = 1
	a.buckets = nil
	a.clock = nil

	assert.Equal(t, 1, b.idx)
	assert.Equal(t, origTime, b.lastRoll)
	assert.Equal(t, 2*time.Second, b.period)
	assert.Equal(t, 3, b.bucketCount)
	assert.Equal(t, int64(4), b.low)
	assert.EqualValues(t, 5, b.high)
	assert.NotNil(t, b.buckets)
	assert.NotNil(t, b.clock)
}
