package memmetrics

import (
	"fmt"
	"time"

	"github.com/codahale/hdrhistogram"
	"github.com/mailgun/timetools"
)

// HDRHistogram is a tiny wrapper around github.com/codahale/hdrhistogram that provides convenience functions for measuring http latencies
type HDRHistogram struct {
	// lowest trackable value
	low int64
	// highest trackable value
	high int64
	// significant figures
	sigfigs int

	h *hdrhistogram.Histogram
}

func NewHDRHistogram(low, high int64, sigfigs int) (h *HDRHistogram, err error) {
	defer func() {
		if msg := recover(); msg != nil {
			err = fmt.Errorf("%s", msg)
		}
	}()

	hdr := hdrhistogram.New(low, high, sigfigs)
	h = &HDRHistogram{
		low:     low,
		high:    high,
		sigfigs: sigfigs,
		h:       hdr,
	}
	return h, err
}

// Returns latency at quantile with microsecond precision
func (h *HDRHistogram) LatencyAtQuantile(q float64) time.Duration {
	return time.Duration(h.ValueAtQuantile(q)) * time.Microsecond
}

// Records latencies with microsecond precision
func (h *HDRHistogram) RecordLatencies(d time.Duration, n int64) error {
	return h.RecordValues(int64(d/time.Microsecond), n)
}

func (h *HDRHistogram) Reset() {
	h.h.Reset()
}

func (h *HDRHistogram) ValueAtQuantile(q float64) int64 {
	return h.h.ValueAtQuantile(q)
}

func (h *HDRHistogram) RecordValues(v, n int64) error {
	return h.h.RecordValues(v, n)
}

func (h *HDRHistogram) Merge(other *HDRHistogram) error {
	h.h.Merge(other.h)
	return nil
}

type rhOptSetter func(r *RollingHDRHistogram) error

func Clock(clock timetools.TimeProvider) rhOptSetter {
	return func(r *RollingHDRHistogram) error {
		r.clock = clock
		return nil
	}
}

func MinValue(low int) rhOptSetter {
	return func(r *RollingHDRHistogram) error {
		r.low = low
		return nil
	}
}

func MaxValue(max int) rhOptSetter {
	return func(r *RollingHDRHistogram) error {
		r.high = high
		return nil
	}
}

func SigFigs(v int) rhOptSetter {
	return func(r *RollingHDRHistogram) error {
		r.sigfigs = sigfigs
		return nil
	}
}

// RollingHistogram holds multiple histograms and rotates every period.
// It provides resulting histogram as a result of a call of 'Merged' function.
type RollingHDRHistogram struct {
	low      int
	high     int
	sigfigs  int
	idx      int
	lastRoll time.Time
	period   time.Duration
	buckets  []*HDRHistogram
	clock    timetools.TimeProvider
}

func NewRollingHDRHistogram(bucketCount int, period time.Duration, settings ...rhOptSetter) (*RollingHDRHistogram, error) {
	rh := &rollingHistogram{
		buckets: buckets,
		period:  period,
	}

	for _, s := range settings {
		if err := s(rh); err != nil {
			return nil, err
		}
	}

	if rh.low == 0 {
		rh.low = histMin
	}

	if rh.high == 0 {
		rh.high = histMax
	}

	if rh.sigfigs == 0 {
		rw.sigfigs = histSignificantFigures
	}

	buckets := make([]Histogram, bucketCount)
	for i := range buckets {
		h, err := NewHDRHistogram(low, high, sigfigs)
		if err != nil {
			return nil, err
		}
		buckets[i] = h
	}

	return rh, nil
}

func (r *RollingHDRHistogram) Reset() {
	r.idx = 0
	r.lastRoll = r.timeProvider.UtcNow()
	for _, b := range r.buckets {
		b.Reset()
	}
}

func (r *RollingHDRHistogram) rotate() {
	r.idx = (r.idx + 1) % len(r.buckets)
	r.buckets[r.idx].Reset()
}

func (r *RollingHDRHistogram) Merged() (*HDRHistogram, error) {
	m, err := r.maker()
	if err != nil {
		return m, err
	}
	for _, h := range r.buckets {
		if m.Merge(h); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (r *RollingHDRHistogram) getHist() *HDRHistogram {
	if r.timeProvider.UtcNow().Sub(r.lastRoll) >= r.period {
		r.rotate()
		r.lastRoll = r.timeProvider.UtcNow()
	}
	return r.buckets[r.idx]
}

func (r *RollingHDRHistogram) RecordLatencies(v time.Duration, n int64) error {
	return r.getHist().RecordLatencies(v, n)
}

func (r *RollingHDRHistogram) RecordValues(v, n int64) error {
	return r.getHist().RecordValues(v, n)
}
