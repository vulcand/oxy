package stream

import (
	"github.com/vulcand/oxy/v2/utils"
)

// Option represents an option you can pass to New.
type Option func(s *Stream) error

// Logger defines the logger used by Stream.
func Logger(l utils.Logger) Option {
	return func(s *Stream) error {
		s.log = l
		return nil
	}
}

// Verbose additional debug information.
func Verbose(verbose bool) Option {
	return func(s *Stream) error {
		s.verbose = verbose
		return nil
	}
}
