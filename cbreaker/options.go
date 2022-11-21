package cbreaker

import (
	"net/http"
	"time"

	"github.com/vulcand/oxy/v2/utils"
)

// Option represents an option you can pass to New.
type Option func(*CircuitBreaker) error

// Logger defines the logger used by CircuitBreaker.
func Logger(l utils.Logger) Option {
	return func(c *CircuitBreaker) error {
		c.log = l
		return nil
	}
}

// Verbose additional debug information.
func Verbose(verbose bool) Option {
	return func(c *CircuitBreaker) error {
		c.verbose = verbose
		return nil
	}
}

// FallbackDuration is how long the CircuitBreaker will remain in the Tripped
// state before trying to recover.
func FallbackDuration(d time.Duration) Option {
	return func(c *CircuitBreaker) error {
		c.fallbackDuration = d
		return nil
	}
}

// RecoveryDuration is how long the CircuitBreaker will take to ramp up
// requests during the Recovering state.
func RecoveryDuration(d time.Duration) Option {
	return func(c *CircuitBreaker) error {
		c.recoveryDuration = d
		return nil
	}
}

// CheckPeriod is how long the CircuitBreaker will wait between successive
// checks of the breaker condition.
func CheckPeriod(d time.Duration) Option {
	return func(c *CircuitBreaker) error {
		c.checkPeriod = d
		return nil
	}
}

// OnTripped sets a SideEffect to run when entering the Tripped state.
// Only one SideEffect can be set for this hook.
func OnTripped(s SideEffect) Option {
	return func(c *CircuitBreaker) error {
		c.onTripped = s
		return nil
	}
}

// OnStandby sets a SideEffect to run when entering the Standby state.
// Only one SideEffect can be set for this hook.
func OnStandby(s SideEffect) Option {
	return func(c *CircuitBreaker) error {
		c.onStandby = s
		return nil
	}
}

// Fallback defines the http.Handler that the CircuitBreaker should route
// requests to when it prevents a request from taking its normal path.
func Fallback(h http.Handler) Option {
	return func(c *CircuitBreaker) error {
		c.fallback = h
		return nil
	}
}

// ResponseFallbackOption represents an option you can pass to NewResponseFallback.
type ResponseFallbackOption func(*ResponseFallback) error

// ResponseFallbackLogger defines the logger used by ResponseFallback.
func ResponseFallbackLogger(l utils.Logger) ResponseFallbackOption {
	return func(c *ResponseFallback) error {
		c.log = l
		return nil
	}
}

// ResponseFallbackDebug additional debug information.
func ResponseFallbackDebug(debug bool) ResponseFallbackOption {
	return func(c *ResponseFallback) error {
		c.debug = debug
		return nil
	}
}

// RedirectFallbackOption represents an option you can pass to NewRedirectFallback.
type RedirectFallbackOption func(*RedirectFallback) error

// RedirectFallbackLogger defines the logger used by ResponseFallback.
func RedirectFallbackLogger(l utils.Logger) RedirectFallbackOption {
	return func(c *RedirectFallback) error {
		c.log = l
		return nil
	}
}

// RedirectFallbackDebug additional debug information.
func RedirectFallbackDebug(debug bool) RedirectFallbackOption {
	return func(c *RedirectFallback) error {
		c.debug = debug
		return nil
	}
}
