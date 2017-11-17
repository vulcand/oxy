package memmetrics

import (
	"runtime"
	"sync"
	"time"

	"github.com/mailgun/timetools"
	. "gopkg.in/check.v1"
)

type RRSuite struct {
	tm *timetools.FreezedTime
}

var _ = Suite(&RRSuite{})

func (s *RRSuite) SetUpSuite(c *C) {
	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func (s *RRSuite) TestDefaults(c *C) {
	rr, err := NewRTMetrics(RTClock(s.tm))
	c.Assert(err, IsNil)
	c.Assert(rr, NotNil)

	rr.Record(200, time.Second)
	rr.Record(502, 2*time.Second)
	rr.Record(200, time.Second)
	rr.Record(200, time.Second)

	c.Assert(rr.NetworkErrorCount(), Equals, int64(1))
	c.Assert(rr.TotalCount(), Equals, int64(4))
	c.Assert(rr.StatusCodesCounts(), DeepEquals, map[int]int64{502: 1, 200: 3})
	c.Assert(rr.NetworkErrorRatio(), Equals, float64(1)/float64(4))
	c.Assert(rr.ResponseCodeRatio(500, 503, 200, 300), Equals, 1.0/3.0)

	h, err := rr.LatencyHistogram()
	c.Assert(err, IsNil)
	c.Assert(int(h.LatencyAtQuantile(100)/time.Second), Equals, 2)

	rr.Reset()
	c.Assert(rr.NetworkErrorCount(), Equals, int64(0))
	c.Assert(rr.TotalCount(), Equals, int64(0))
	c.Assert(rr.StatusCodesCounts(), DeepEquals, map[int]int64{})
	c.Assert(rr.NetworkErrorRatio(), Equals, float64(0))
	c.Assert(rr.ResponseCodeRatio(500, 503, 200, 300), Equals, float64(0))

	h, err = rr.LatencyHistogram()
	c.Assert(err, IsNil)
	c.Assert(h.LatencyAtQuantile(100), Equals, time.Duration(0))

}

func (s *RRSuite) TestAppend(c *C) {
	rr, err := NewRTMetrics(RTClock(s.tm))
	c.Assert(err, IsNil)
	c.Assert(rr, NotNil)

	rr.Record(200, time.Second)
	rr.Record(502, 2*time.Second)
	rr.Record(200, time.Second)
	rr.Record(200, time.Second)

	rr2, err := NewRTMetrics(RTClock(s.tm))
	c.Assert(err, IsNil)
	c.Assert(rr2, NotNil)

	rr2.Record(200, 3*time.Second)
	rr2.Record(501, 3*time.Second)
	rr2.Record(200, 3*time.Second)
	rr2.Record(200, 3*time.Second)

	c.Assert(rr2.Append(rr), IsNil)
	c.Assert(rr2.StatusCodesCounts(), DeepEquals, map[int]int64{501: 1, 502: 1, 200: 6})
	c.Assert(rr2.NetworkErrorCount(), Equals, int64(1))

	h, err := rr2.LatencyHistogram()
	c.Assert(err, IsNil)
	c.Assert(int(h.LatencyAtQuantile(100)/time.Second), Equals, 3)
}

func (s *RRSuite) TestConcurrentRecords(c *C) {
	// This test asserts a race condition which requires parallelism
	runtime.GOMAXPROCS(100)

	rr, _ := NewRTMetrics(RTClock(s.tm))

	for code := 0; code < 100; code++ {
		for numRecords := 0; numRecords < 10; numRecords++ {
			go func(statusCode int) {
				rr.recordStatusCode(statusCode)
			}(code)
		}
	}
}

func (s *RRSuite) TestRTMetricExportReturnsNewCopy(c *C) {
	a := RTMetrics{}
	a.clock = &timetools.RealTime{}
	a.total, _ = NewCounter(1, time.Second, CounterClock(a.clock))
	a.netErrors, _ = NewCounter(1, time.Second, CounterClock(a.clock))
	a.statusCodes = map[int]*RollingCounter{}
	a.statusCodesLock = sync.RWMutex{}
	a.histogram = &RollingHDRHistogram{}
	a.histogramLock = sync.RWMutex{}
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

	c.Assert(b.total, NotNil)
	c.Assert(b.netErrors, NotNil)
	c.Assert(b.statusCodes, NotNil)
	c.Assert(b.histogram, NotNil)
	c.Assert(b.newCounter, NotNil)
	c.Assert(b.newHist, NotNil)
	c.Assert(b.clock, NotNil)

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
		case <-time.After(10 * time.Second):
			c.FailNow()
		}
	}
}
