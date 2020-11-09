package memmetrics

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/mailgun/timetools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/oxy/testutils"
)

func BenchmarkRecord(b *testing.B) {
	b.ReportAllocs()

	rr, err := NewRTMetrics(RTClock(testutils.GetClock()))
	require.NoError(b, err)

	// warm up metrics. Adding a new code can do allocations, but in the steady
	// state recording a code is cheap. We want to measure the steady state.
	const codes = 100
	for code := 0; code < codes; code++ {
		rr.Record(code, time.Second)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr.Record(i%codes, time.Second)
	}
}

func BenchmarkRecordConcurrently(b *testing.B) {
	b.ReportAllocs()

	rr, err := NewRTMetrics(RTClock(testutils.GetClock()))
	require.NoError(b, err)

	// warm up metrics. Adding a new code can do allocations, but in the steady
	// state recording a code is cheap. We want to measure the steady state.
	const codes = 100
	for code := 0; code < codes; code++ {
		rr.Record(code, time.Second)
	}

	concurrency := runtime.NumCPU()
	b.Logf("NumCPU: %d, Concurrency: %d, GOMAXPROCS: %d",
		runtime.NumCPU(), concurrency, runtime.GOMAXPROCS(0))
	wg := sync.WaitGroup{}
	wg.Add(concurrency)
	perG := b.N/concurrency
	if perG == 0 {
		perG = 1
	}

	b.ResetTimer()
	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < perG; j++ {
				rr.Record(j%codes, time.Second)
			}
			wg.Done()
		}()
	}

	wg.Wait()
}

func TestDefaults(t *testing.T) {
	rr, err := NewRTMetrics(RTClock(testutils.GetClock()))
	require.NoError(t, err)
	require.NotNil(t, rr)

	rr.Record(200, time.Second)
	rr.Record(502, 2*time.Second)
	rr.Record(200, time.Second)
	rr.Record(200, time.Second)

	assert.EqualValues(t, 1, rr.NetworkErrorCount())
	assert.EqualValues(t, 4, rr.TotalCount())
	assert.Equal(t, map[int]int64{502: 1, 200: 3}, rr.StatusCodesCounts())
	assert.Equal(t, float64(1)/float64(4), rr.NetworkErrorRatio())
	assert.Equal(t, 1.0/3.0, rr.ResponseCodeRatio(500, 503, 200, 300))

	h, err := rr.LatencyHistogram()
	require.NoError(t, err)
	assert.Equal(t, 2, int(h.LatencyAtQuantile(100)/time.Second))

	rr.Reset()
	assert.EqualValues(t, 0, rr.NetworkErrorCount())
	assert.EqualValues(t, 0, rr.TotalCount())
	assert.Equal(t, map[int]int64{}, rr.StatusCodesCounts())
	assert.Equal(t, float64(0), rr.NetworkErrorRatio())
	assert.Equal(t, float64(0), rr.ResponseCodeRatio(500, 503, 200, 300))

	h, err = rr.LatencyHistogram()
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), h.LatencyAtQuantile(100))
}

func TestAppend(t *testing.T) {
	clock := testutils.GetClock()

	rr, err := NewRTMetrics(RTClock(clock))
	require.NoError(t, err)
	require.NotNil(t, rr)

	rr.Record(200, time.Second)
	rr.Record(502, 2*time.Second)
	rr.Record(200, time.Second)
	rr.Record(200, time.Second)

	rr2, err := NewRTMetrics(RTClock(clock))
	require.NoError(t, err)
	require.NotNil(t, rr2)

	rr2.Record(200, 3*time.Second)
	rr2.Record(501, 3*time.Second)
	rr2.Record(200, 3*time.Second)
	rr2.Record(200, 3*time.Second)

	require.NoError(t, rr2.Append(rr))
	assert.Equal(t, map[int]int64{501: 1, 502: 1, 200: 6}, rr2.StatusCodesCounts())
	assert.EqualValues(t, 1, rr2.NetworkErrorCount())

	h, err := rr2.LatencyHistogram()
	require.NoError(t, err)
	assert.EqualValues(t, 3, h.LatencyAtQuantile(100)/time.Second)
}

func TestConcurrentRecords(t *testing.T) {
	// This test asserts a race condition which requires concurrency. Set
	// GOMAXPROCS high for this test, then restore after test completes.
	n := runtime.GOMAXPROCS(0)
	runtime.GOMAXPROCS(100)
	defer runtime.GOMAXPROCS(n)

	rr, err := NewRTMetrics(RTClock(testutils.GetClock()))
	require.NoError(t, err)

	for code := 0; code < 100; code++ {
		for numRecords := 0; numRecords < 10; numRecords++ {
			go func(statusCode int) {
				rr.Record(statusCode, time.Second)
			}(code)
		}
	}
}

func TestRTMetricExportReturnsNewCopy(t *testing.T) {
	a := RTMetrics{
		clock:       &timetools.RealTime{},
		statusCodes: map[int]*RollingCounter{},
		histogram:   &RollingHDRHistogram{},
	}

	var err error
	a.total, err = NewCounter(1, time.Second, CounterClock(a.clock))
	require.NoError(t, err)

	a.netErrors, err = NewCounter(1, time.Second, CounterClock(a.clock))
	require.NoError(t, err)

	a.newCounter = func() (*RollingCounter, error) {
		return NewCounter(counterBuckets, counterResolution, CounterClock(a.clock))
	}
	a.newHist = func() (*RollingHDRHistogram, error) {
		return NewRollingHDRHistogram(histMin, histMax, histSignificantFigures, histPeriod, histBuckets, RollingClock(a.clock))
	}

	b := a.Export()
	a.total = nil
	a.netErrors = nil
	a.statusCodes = nil
	a.histogram = nil
	a.newCounter = nil
	a.newHist = nil
	a.clock = nil

	assert.NotNil(t, b.total)
	assert.NotNil(t, b.netErrors)
	assert.NotNil(t, b.statusCodes)
	assert.NotNil(t, b.histogram)
	assert.NotNil(t, b.newCounter)
	assert.NotNil(t, b.newHist)
	assert.NotNil(t, b.clock)
}
