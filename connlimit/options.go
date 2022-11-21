package connlimit

import (
	"github.com/vulcand/oxy/v2/utils"
)

// Option represents an option you can pass to New.
type Option func(l *ConnLimiter) error

// Logger defines the logger used by ConnLimiter.
func Logger(l utils.Logger) Option {
	return func(cl *ConnLimiter) error {
		cl.log = l
		return nil
	}
}

// Verbose additional debug information.
func Verbose(verbose bool) Option {
	return func(cl *ConnLimiter) error {
		cl.verbose = verbose
		return nil
	}
}

// ErrorHandler sets error handler of the server.
func ErrorHandler(h utils.ErrorHandler) Option {
	return func(cl *ConnLimiter) error {
		cl.errHandler = h
		return nil
	}
}
