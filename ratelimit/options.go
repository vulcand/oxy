package ratelimit

import (
	"fmt"

	"github.com/vulcand/oxy/v2/utils"
)

// TokenLimiterOption token limiter option type.
type TokenLimiterOption func(l *TokenLimiter) error

// ErrorHandler sets error handler of the server.
func ErrorHandler(h utils.ErrorHandler) TokenLimiterOption {
	return func(cl *TokenLimiter) error {
		cl.errHandler = h
		return nil
	}
}

// ExtractRates sets the rate extractor.
func ExtractRates(e RateExtractor) TokenLimiterOption {
	return func(cl *TokenLimiter) error {
		cl.extractRates = e
		return nil
	}
}

// Capacity sets the capacity.
func Capacity(capacity int) TokenLimiterOption {
	return func(cl *TokenLimiter) error {
		if capacity <= 0 {
			return fmt.Errorf("bad capacity: %v", capacity)
		}
		cl.capacity = capacity
		return nil
	}
}

// Logger defines the logger the TokenLimiter will use.
func Logger(l utils.Logger) TokenLimiterOption {
	return func(tl *TokenLimiter) error {
		tl.log = l
		return nil
	}
}
