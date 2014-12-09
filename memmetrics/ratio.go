package memmetrics

import (
	"time"

	"github.com/mailgun/timetools"
)

// RatioCounter calculates a ratio of a/a+b over a rolling window of predefined buckets
type RatioCounter struct {
	a *RollingCounter
	b *RollingCounter
}

func NewRatioCounter(buckets int, resolution time.Duration, timeProvider timetools.TimeProvider) (*RatioCounter, error) {
	a, err := NewCounter(buckets, resolution, timeProvider)
	if err != nil {
		return nil, err
	}

	b, err := NewCounter(buckets, resolution, timeProvider)
	if err != nil {
		return nil, err
	}

	return &RatioCounter{
		a: a,
		b: b,
	}, nil
}

func (r *RatioCounter) Reset() {
	r.a.Reset()
	r.b.Reset()
}

func (r *RatioCounter) IsReady() bool {
	return r.a.countedBuckets+r.b.countedBuckets >= len(r.a.values)
}

func (r *RatioCounter) CountA() int64 {
	return r.a.Count()
}

func (r *RatioCounter) CountB() int64 {
	return r.b.Count()
}

func (r *RatioCounter) Resolution() time.Duration {
	return r.a.Resolution()
}

func (r *RatioCounter) Buckets() int {
	return r.a.Buckets()
}

func (r *RatioCounter) WindowSize() time.Duration {
	return r.a.WindowSize()
}

func (r *RatioCounter) ProcessedCount() int64 {
	return r.CountA() + r.CountB()
}

func (r *RatioCounter) Ratio() float64 {
	a := r.a.Count()
	b := r.b.Count()
	// No data, return ok
	if a+b == 0 {
		return 0
	}
	return float64(a) / float64(a+b)
}

func (r *RatioCounter) IncA() {
	r.a.Inc()
}

func (r *RatioCounter) IncB() {
	r.b.Inc()
}

type TestMeter struct {
	Rate       float64
	NotReady   bool
	WindowSize time.Duration
}

func (tm *TestMeter) GetWindowSize() time.Duration {
	return tm.WindowSize
}

func (tm *TestMeter) IsReady() bool {
	return !tm.NotReady
}

func (tm *TestMeter) GetRate() float64 {
	return tm.Rate
}
