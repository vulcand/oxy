package memmetrics

import (
	"testing"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
	"github.com/vulcand/oxy/v2/testutils"
)

func TestHDRHistogram_Merge(t *testing.T) {
	a, err := NewHDRHistogram(1, 3600000, 2)
	require.NoError(t, err)

	require.NoError(t, a.RecordValues(1, 2))

	b, err := NewHDRHistogram(1, 3600000, 2)
	require.NoError(t, err)

	require.NoError(t, b.RecordValues(2, 1))

	err = a.Merge(b)
	require.NoError(t, err)

	assert.EqualValues(t, 1, a.ValueAtQuantile(50))
	assert.EqualValues(t, 2, a.ValueAtQuantile(100))
}

func TestHDRHistogram_Merge_nil(t *testing.T) {
	a, err := NewHDRHistogram(1, 3600000, 1)
	require.NoError(t, err)

	require.Error(t, a.Merge(nil))
}

func TestHDRHistogram_rotation(t *testing.T) {
	testutils.FreezeTime(t)

	h, err := NewRollingHDRHistogram(
		1,       // min value
		3600000, // max value
		3,       // significant figures
		clock.Second,
		2, // 2 histograms in a window
	)

	require.NoError(t, err)
	require.NotNil(t, h)

	err = h.RecordValues(5, 1)
	require.NoError(t, err)

	m, err := h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 5, m.ValueAtQuantile(100))

	clock.Advance(clock.Second)
	require.NoError(t, h.RecordValues(2, 1))
	require.NoError(t, h.RecordValues(1, 1))

	m, err = h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 5, m.ValueAtQuantile(100))

	// rotate, this means that the old value would evaporate
	clock.Advance(clock.Second)

	require.NoError(t, h.RecordValues(1, 1))

	m, err = h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 2, m.ValueAtQuantile(100))
}

func TestHDRHistogram_Reset(t *testing.T) {
	testutils.FreezeTime(t)

	h, err := NewRollingHDRHistogram(
		1,       // min value
		3600000, // max value
		3,       // significant figures
		clock.Second,
		2, // 2 histograms in a window
	)

	require.NoError(t, err)
	require.NotNil(t, h)

	require.NoError(t, h.RecordValues(5, 1))

	m, err := h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 5, m.ValueAtQuantile(100))

	clock.Advance(clock.Second)
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

	clock.Advance(clock.Second)
	require.NoError(t, h.RecordValues(2, 1))
	require.NoError(t, h.RecordValues(1, 1))

	m, err = h.Merged()
	require.NoError(t, err)
	assert.EqualValues(t, 5, m.ValueAtQuantile(100))
}

func TestHDRHistogram_Export_returnsNewCopy(t *testing.T) {
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

func TestRollingHDRHistogram_Export_returnsNewCopy(t *testing.T) {
	origTime := clock.Now()

	testutils.FreezeTime(t)

	a := RollingHDRHistogram{
		idx:         1,
		lastRoll:    origTime,
		period:      2 * clock.Second,
		bucketCount: 3,
		low:         4,
		high:        5,
		sigfigs:     1,
		buckets:     []*HDRHistogram{},
	}

	b := a.Export()
	a.idx = 11
	a.lastRoll = clock.Now().Add(1 * clock.Minute)
	a.period = 12 * clock.Second
	a.bucketCount = 13
	a.low = 14
	a.high = 15
	a.sigfigs = 1
	a.buckets = nil

	assert.Equal(t, 1, b.idx)
	assert.Equal(t, origTime, b.lastRoll)
	assert.Equal(t, 2*clock.Second, b.period)
	assert.Equal(t, 3, b.bucketCount)
	assert.Equal(t, int64(4), b.low)
	assert.EqualValues(t, 5, b.high)
	assert.NotNil(t, b.buckets)
}
