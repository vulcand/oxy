package memmetrics

import (
	"errors"
	"fmt"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
)

// HDRHistogram is a tiny wrapper around github.com/HdrHistogram/hdrhistogram-go that provides convenience functions for measuring http latencies.
type HDRHistogram struct {
	// lowest trackable value
	low int64
	// highest trackable value
	high int64
	// significant figures
	sigfigs int

	h *hdrhistogram.Histogram
}

// NewHDRHistogram creates a new HDRHistogram.
func NewHDRHistogram(low, high int64, sigfigs int) (h *HDRHistogram, err error) {
	defer func() {
		if msg := recover(); msg != nil {
			err = fmt.Errorf("%s", msg)
		}
	}()

	return &HDRHistogram{
		low:     low,
		high:    high,
		sigfigs: sigfigs,
		h:       hdrhistogram.New(low, high, sigfigs),
	}, nil
}

// Export exports a HDRHistogram.
func (h *HDRHistogram) Export() *HDRHistogram {
	var hist *hdrhistogram.Histogram

	if h.h != nil {
		snapshot := h.h.Export()
		hist = hdrhistogram.Import(snapshot)
	}

	return &HDRHistogram{low: h.low, high: h.high, sigfigs: h.sigfigs, h: hist}
}

// LatencyAtQuantile sets latency at quantile with microsecond precision.
func (h *HDRHistogram) LatencyAtQuantile(q float64) time.Duration {
	return time.Duration(h.ValueAtQuantile(q)) * clock.Microsecond
}

// RecordLatencies Records latencies with microsecond precision.
func (h *HDRHistogram) RecordLatencies(d time.Duration, n int64) error {
	return h.RecordValues(int64(d/clock.Microsecond), n)
}

// Reset resets a HDRHistogram.
func (h *HDRHistogram) Reset() {
	h.h.Reset()
}

// ValueAtQuantile sets value at quantile.
func (h *HDRHistogram) ValueAtQuantile(q float64) int64 {
	return h.h.ValueAtQuantile(q)
}

// RecordValues sets record values.
func (h *HDRHistogram) RecordValues(v, n int64) error {
	return h.h.RecordValues(v, n)
}

// Merge merges a HDRHistogram.
func (h *HDRHistogram) Merge(other *HDRHistogram) error {
	if other == nil {
		return errors.New("other is nil")
	}

	h.h.Merge(other.h)

	return nil
}

type rhOption func(r *RollingHDRHistogram) error

// RollingHDRHistogram holds multiple histograms and rotates every period.
// It provides resulting histogram as a result of a call of 'Merged' function.
type RollingHDRHistogram struct {
	idx         int
	lastRoll    clock.Time
	period      time.Duration
	bucketCount int
	low         int64
	high        int64
	sigfigs     int
	buckets     []*HDRHistogram
}

// NewRollingHDRHistogram created a new RollingHDRHistogram.
func NewRollingHDRHistogram(low, high int64, sigfigs int, period time.Duration, bucketCount int, options ...rhOption) (*RollingHDRHistogram, error) {
	rh := &RollingHDRHistogram{
		bucketCount: bucketCount,
		period:      period,
		low:         low,
		high:        high,
		sigfigs:     sigfigs,
	}

	for _, o := range options {
		if err := o(rh); err != nil {
			return nil, err
		}
	}

	buckets := make([]*HDRHistogram, rh.bucketCount)
	for i := range buckets {
		h, err := NewHDRHistogram(low, high, sigfigs)
		if err != nil {
			return nil, err
		}

		buckets[i] = h
	}

	rh.buckets = buckets

	return rh, nil
}

// Export exports a RollingHDRHistogram.
func (r *RollingHDRHistogram) Export() *RollingHDRHistogram {
	export := &RollingHDRHistogram{}
	export.idx = r.idx
	export.lastRoll = r.lastRoll
	export.period = r.period
	export.bucketCount = r.bucketCount
	export.low = r.low
	export.high = r.high
	export.sigfigs = r.sigfigs

	exportBuckets := make([]*HDRHistogram, len(r.buckets))
	for i, hist := range r.buckets {
		exportBuckets[i] = hist.Export()
	}

	export.buckets = exportBuckets

	return export
}

// Append appends a RollingHDRHistogram.
func (r *RollingHDRHistogram) Append(o *RollingHDRHistogram) error {
	if r.bucketCount != o.bucketCount || r.period != o.period || r.low != o.low || r.high != o.high || r.sigfigs != o.sigfigs {
		return errors.New("can't merge")
	}

	for i := range r.buckets {
		if err := r.buckets[i].Merge(o.buckets[i]); err != nil {
			return err
		}
	}

	return nil
}

// Reset resets a RollingHDRHistogram.
func (r *RollingHDRHistogram) Reset() {
	r.idx = 0

	r.lastRoll = clock.Now().UTC()
	for _, b := range r.buckets {
		b.Reset()
	}
}

// Merged gets merged histogram.
func (r *RollingHDRHistogram) Merged() (*HDRHistogram, error) {
	m, err := NewHDRHistogram(r.low, r.high, r.sigfigs)
	if err != nil {
		return m, err
	}

	for _, h := range r.buckets {
		if errMerge := m.Merge(h); errMerge != nil {
			return nil, errMerge
		}
	}

	return m, nil
}

// RecordLatencies sets records latencies.
func (r *RollingHDRHistogram) RecordLatencies(v time.Duration, n int64) error {
	return r.getHist().RecordLatencies(v, n)
}

// RecordValues sets record values.
func (r *RollingHDRHistogram) RecordValues(v, n int64) error {
	return r.getHist().RecordValues(v, n)
}

func (r *RollingHDRHistogram) rotate() {
	r.idx = (r.idx + 1) % len(r.buckets)
	r.buckets[r.idx].Reset()
}

func (r *RollingHDRHistogram) getHist() *HDRHistogram {
	if clock.Now().UTC().Sub(r.lastRoll) >= r.period {
		r.rotate()
		r.lastRoll = clock.Now().UTC()
	}

	return r.buckets[r.idx]
}
