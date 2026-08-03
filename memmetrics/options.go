package memmetrics

import "time"

// RTOption represents an option you can pass to NewRTMetrics.
type RTOption func(r *RTMetrics) error

// RTCounter set a builder function for Counter.
func RTCounter(fn NewCounterFn) RTOption {
	return func(r *RTMetrics) error {
		r.newCounter = fn
		return nil
	}
}

// RTHistogram set a builder function for RollingHDRHistogram.
func RTHistogram(fn NewRollingHistogramFn) RTOption {
	return func(r *RTMetrics) error {
		r.newHist = fn
		return nil
	}
}

// WithRTSlidingWindow sets shared sliding window config for RTMetrics counter and HDR histogram.
// window: total duration of sliding statistics window.
// interval: bucket rotation period, matches RollingCounter resolution and RollingHDRHistogram period.
func WithRTSlidingWindow(window, interval time.Duration) RTOption {
	return func(m *RTMetrics) error {
		m.windowSize = window
		m.slideInterval = interval
		return nil
	}
}

// RatioOption represents an option you can pass to NewRatioCounter.
type RatioOption func(r *RatioCounter) error
