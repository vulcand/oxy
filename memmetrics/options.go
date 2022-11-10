package memmetrics

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

// RatioOption represents an option you can pass to NewRatioCounter.
type RatioOption func(r *RatioCounter) error
