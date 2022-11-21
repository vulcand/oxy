package buffer

import (
	"fmt"

	"github.com/vulcand/oxy/v2/utils"
)

// Option represents an option you can pass to New.
type Option func(b *Buffer) error

// Logger defines the logger used by Buffer.
func Logger(l utils.Logger) Option {
	return func(b *Buffer) error {
		b.log = l
		return nil
	}
}

// Verbose additional debug information.
func Verbose(verbose bool) Option {
	return func(b *Buffer) error {
		b.verbose = verbose
		return nil
	}
}

// Cond Conditional setter.
// ex: Cond(a > 4, MemRequestBodyBytes(a))
func Cond(condition bool, setter Option) Option {
	if !condition {
		// NoOp setter
		return func(*Buffer) error {
			return nil
		}
	}
	return setter
}

// Retry provides a predicate that allows buffer middleware to replay the request
// if it matches certain condition, e.g. returns special error code. Available functions are:
//
// Attempts() - limits the amount of retry attempts
// ResponseCode() - returns http response code
// IsNetworkError() - tests if response code is related to networking error
//
// Example of the predicate:
//
// `Attempts() <= 2 && ResponseCode() == 502`.
func Retry(predicate string) Option {
	return func(b *Buffer) error {
		p, err := parseExpression(predicate)
		if err != nil {
			return err
		}
		b.retryPredicate = p
		return nil
	}
}

// ErrorHandler sets error handler of the server.
func ErrorHandler(h utils.ErrorHandler) Option {
	return func(b *Buffer) error {
		b.errHandler = h
		return nil
	}
}

// MaxRequestBodyBytes sets the maximum request body size in bytes.
func MaxRequestBodyBytes(m int64) Option {
	return func(b *Buffer) error {
		if m < 0 {
			return fmt.Errorf("max bytes should be >= 0 got %d", m)
		}
		b.maxRequestBodyBytes = m
		return nil
	}
}

// MemRequestBodyBytes bytes sets the maximum request body to be stored in memory
// buffer middleware will serialize the excess to disk.
func MemRequestBodyBytes(m int64) Option {
	return func(b *Buffer) error {
		if m < 0 {
			return fmt.Errorf("mem bytes should be >= 0 got %d", m)
		}
		b.memRequestBodyBytes = m
		return nil
	}
}

// MaxResponseBodyBytes sets the maximum response body size in bytes.
func MaxResponseBodyBytes(m int64) Option {
	return func(b *Buffer) error {
		if m < 0 {
			return fmt.Errorf("max bytes should be >= 0 got %d", m)
		}
		b.maxResponseBodyBytes = m
		return nil
	}
}

// MemResponseBodyBytes sets the maximum response body to be stored in memory
// buffer middleware will serialize the excess to disk.
func MemResponseBodyBytes(m int64) Option {
	return func(b *Buffer) error {
		if m < 0 {
			return fmt.Errorf("mem bytes should be >= 0 got %d", m)
		}
		b.memResponseBodyBytes = m
		return nil
	}
}
