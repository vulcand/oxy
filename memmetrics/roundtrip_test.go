package memmetrics

import (
	"runtime"
	"sync"
	"testing"

	"github.com/mailgun/holster/v4/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
)

func TestDefaults(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	rr, err := NewRTMetrics()
	require.NoError(t, err)
	require.NotNil(t, rr)

	rr.Record(200, clock.Second)
	rr.Record(502, 2*clock.Second)
	rr.Record(200, clock.Second)
	rr.Record(200, clock.Second)

	assert.EqualValues(t, 1, rr.NetworkErrorCount())
	assert.EqualValues(t, 4, rr.TotalCount())
	assert.Equal(t, map[int]int64{502: 1, 200: 3}, rr.StatusCodesCounts())
	assert.Equal(t, float64(1)/float64(4), rr.NetworkErrorRatio())
	assert.Equal(t, 1.0/3.0, rr.ResponseCodeRatio(500, 503, 200, 300))

	h, err := rr.LatencyHistogram()
	require.NoError(t, err)
	assert.Equal(t, 2, int(h.LatencyAtQuantile(100)/clock.Second))

	rr.Reset()
	assert.EqualValues(t, 0, rr.NetworkErrorCount())
	assert.EqualValues(t, 0, rr.TotalCount())
	assert.Equal(t, map[int]int64{}, rr.StatusCodesCounts())
	assert.Equal(t, float64(0), rr.NetworkErrorRatio())
	assert.Equal(t, float64(0), rr.ResponseCodeRatio(500, 503, 200, 300))

	h, err = rr.LatencyHistogram()
	require.NoError(t, err)
	assert.Equal(t, clock.Duration(0), h.LatencyAtQuantile(100))
}

func TestAppend(t *testing.T) {
	done := testutils.FreezeTime()
	defer done()

	rr, err := NewRTMetrics()
	require.NoError(t, err)
	require.NotNil(t, rr)

	rr.Record(200, clock.Second)
	rr.Record(502, 2*clock.Second)
	rr.Record(200, clock.Second)
	rr.Record(200, clock.Second)

	rr2, err := NewRTMetrics()
	require.NoError(t, err)
	require.NotNil(t, rr2)

	rr2.Record(200, 3*clock.Second)
	rr2.Record(501, 3*clock.Second)
	rr2.Record(200, 3*clock.Second)
	rr2.Record(200, 3*clock.Second)

	require.NoError(t, rr2.Append(rr))
	assert.Equal(t, map[int]int64{501: 1, 502: 1, 200: 6}, rr2.StatusCodesCounts())
	assert.EqualValues(t, 1, rr2.NetworkErrorCount())

	h, err := rr2.LatencyHistogram()
	require.NoError(t, err)
	assert.EqualValues(t, 3, h.LatencyAtQuantile(100)/clock.Second)
}

func TestConcurrentRecords(t *testing.T) {
	// This test asserts a race condition which requires parallelism
	runtime.GOMAXPROCS(100)

	rr, err := NewRTMetrics()
	require.NoError(t, err)

	for code := 0; code < 100; code++ {
		for numRecords := 0; numRecords < 10; numRecords++ {
			go func(statusCode int) {
				_ = rr.recordStatusCode(statusCode)
			}(code)
		}
	}
}

func TestRTMetricExportReturnsNewCopy(t *testing.T) {
	a := RTMetrics{
		statusCodes:     map[int]*RollingCounter{},
		statusCodesLock: sync.RWMutex{},
		histogram:       &RollingHDRHistogram{},
		histogramLock:   sync.RWMutex{},
	}

	var err error
	a.total, err = NewCounter(1, clock.Second)
	require.NoError(t, err)

	a.netErrors, err = NewCounter(1, clock.Second)
	require.NoError(t, err)

	a.newCounter = func() (*RollingCounter, error) {
		return NewCounter(counterBuckets, counterResolution)
	}
	a.newHist = func() (*RollingHDRHistogram, error) {
		return NewRollingHDRHistogram(histMin, histMax, histSignificantFigures, histPeriod, histBuckets)
	}

	b := a.Export()
	a.total = nil
	a.netErrors = nil
	a.statusCodes = nil
	a.histogram = nil
	a.newCounter = nil
	a.newHist = nil

	assert.NotNil(t, b.total)
	assert.NotNil(t, b.netErrors)
	assert.NotNil(t, b.statusCodes)
	assert.NotNil(t, b.histogram)
	assert.NotNil(t, b.newCounter)
	assert.NotNil(t, b.newHist)

	// a and b should have different locks
	locksSucceed := make(chan bool)
	go func() {
		a.statusCodesLock.Lock()
		b.statusCodesLock.Lock()
		a.histogramLock.Lock()
		b.histogramLock.Lock()
		locksSucceed <- true
	}()

	for {
		select {
		case <-locksSucceed:
			return
		case <-clock.After(10 * clock.Second):
			t.FailNow()
		}
	}
}
