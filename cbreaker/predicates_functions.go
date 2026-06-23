package cbreaker

import (
	"github.com/vulcand/oxy/v2/internal/holsterv4/clock"
)

type toType[T any] func(c *CircuitBreaker) T

func latencyAtQuantile(quantile float64) toType[int] {
	return func(c *CircuitBreaker) int {
		h, err := c.metrics.LatencyHistogram()
		if err != nil {
			c.log.Error("Failed to get latency histogram, for %v error: %v", c, err)
			return 0
		}

		return int(h.LatencyAtQuantile(quantile) / clock.Millisecond)
	}
}

func networkErrorRatio() toType[float64] {
	return func(c *CircuitBreaker) float64 {
		return c.metrics.NetworkErrorRatio()
	}
}

func responseCodeRatio(startA, endA, startB, endB int) toType[float64] {
	return func(c *CircuitBreaker) float64 {
		return c.metrics.ResponseCodeRatio(startA, endA, startB, endB)
	}
}
