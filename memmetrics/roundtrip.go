package memmetrics

import (
	"net"
	"time"

	"github.com/mailgun/timetools"
)

// RoundTripMetrics provides aggregated performance metrics for HTTP requests processing
// such as round trip latency, response codes counters network error and total requests.
// all counters are collected as rolling window counters with defined precision, histograms
// are a rolling window histograms with defined precision as well.
// See RoundTripOptions for more detail on parameters.
type RoundTripMetrics struct {
	total       *RollingCounter
	netErrors   *RollingCounter
	statusCodes map[int]*RollingCounter
	histogram   *RollingHDRHistogram

	newCounter NewCounterFn
	newHist    NewRollingHistogramFn
	clock      timetools.TimeProvider
}

type rrOptSetter func(r *RoundTripMetrics) error

type NewCounterFn func() (*RollingCounter, error)
type NewRollingHistogramFn func() (*RollingHDRHistogram, error)

func RoundTripCounter(new NewCounterFn) rrOptSetter {
	return func(r *RoundTripMetrics) error {
		r.newCounter = new
		return nil
	}
}

func RoundTripHistogram(new NewRollingHistogramFn) rrOptSetter {
	return func(r *RoundTripMetrics) error {
		r.newHist = new
		return nil
	}
}

func RoundTripClock(clock timetools.TimeProvider) rrOptSetter {
	return func(r *RoundTripMetrics) error {
		r.clock = clock
		return nil
	}
}

// NewRoundTripMetrics returns new instance of metrics collector.
func NewRoundTripMetrics(settings ...rrOptSetter) (*RoundTripMetrics, error) {
	m := &RoundTripMetrics{
		statusCodes: make(map[int]*RollingCounter),
	}
	for _, s := range settings {
		if err := s(m); err != nil {
			return nil, err
		}
	}

	if m.clock == nil {
		m.clock = &timetools.RealTime{}
	}

	if m.newCounter == nil {
		m.newCounter = func() (*RollingCounter, error) {
			return NewCounter(counterBuckets, counterResolution, CounterClock(m.clock))
		}
	}

	if m.newHist == nil {
		m.newHist = func() (*RollingHDRHistogram, error) {
			return NewRollingHDRHistogram(histMin, histMax, histSignificantFigures, histPeriod, histBuckets, RollingClock(m.clock))
		}
	}

	h, err := m.newHist()
	if err != nil {
		return nil, err
	}

	netErrors, err := m.newCounter()
	if err != nil {
		return nil, err
	}

	total, err := m.newCounter()
	if err != nil {
		return nil, err
	}

	m.histogram = h
	m.netErrors = netErrors
	m.total = total
	return m, nil
}

// GetNetworkErrorRatio calculates the amont of network errors such as time outs and dropped connection
// that occured in the given time window compared to the total requests count.
func (m *RoundTripMetrics) NetworkErrorRatio() float64 {
	if m.total.Count() == 0 {
		return 0
	}
	return float64(m.netErrors.Count()) / float64(m.total.Count())
}

// GetResponseCodeRatio calculates ratio of count(startA to endA) / count(startB to endB)
func (m *RoundTripMetrics) ResponseCodeRatio(startA, endA, startB, endB int) float64 {
	a := int64(0)
	b := int64(0)
	for code, v := range m.statusCodes {
		if code < endA && code >= startA {
			a += v.Count()
		}
		if code < endB && code >= startB {
			b += v.Count()
		}
	}
	if b != 0 {
		return float64(a) / float64(b)
	}
	return 0
}

func (m *RoundTripMetrics) Record(code int, err error, duration time.Duration) {
	m.total.Inc()
	if _, ok := err.(net.Error); ok {
		m.netErrors.Inc()
	}
	m.recordStatusCode(code)
	m.recordLatency(duration)
}

// GetTotalCount returns total count of processed requests collected.
func (m *RoundTripMetrics) TotalCount() int64 {
	return m.total.Count()
}

// GetNetworkErrorCount returns total count of processed requests observed
func (m *RoundTripMetrics) NetworkErrorCount() int64 {
	return m.netErrors.Count()
}

// GetStatusCodesCounts returns map with counts of the response codes
func (m *RoundTripMetrics) StatusCodesCounts() map[int]int64 {
	sc := make(map[int]int64)
	for k, v := range m.statusCodes {
		if v.Count() != 0 {
			sc[k] = v.Count()
		}
	}
	return sc
}

// GetLatencyHistogram computes and returns resulting histogram with latencies observed.
func (m *RoundTripMetrics) LatencyHistogram() (*HDRHistogram, error) {
	return m.histogram.Merged()
}

func (m *RoundTripMetrics) Reset() {
	m.histogram.Reset()
	m.total.Reset()
	m.netErrors.Reset()
	m.statusCodes = make(map[int]*RollingCounter)
}

func (m *RoundTripMetrics) recordNetError() error {
	m.netErrors.Inc()
	return nil
}

func (m *RoundTripMetrics) recordLatency(d time.Duration) error {
	return m.histogram.RecordLatencies(d, 1)
}

func (m *RoundTripMetrics) recordStatusCode(statusCode int) error {
	if c, ok := m.statusCodes[statusCode]; ok {
		c.Inc()
		return nil
	}
	c, err := m.newCounter()
	if err != nil {
		return err
	}
	c.Inc()
	m.statusCodes[statusCode] = c
	return nil
}

const (
	counterBuckets         = 10
	counterResolution      = time.Second
	histMin                = 1
	histMax                = 3600000000       // 1 hour in microseconds
	histSignificantFigures = 2                // signigicant figures (1% precision)
	histBuckets            = 6                // number of sub-histograms in a rolling histogram
	histPeriod             = 10 * time.Second // roll time
)
