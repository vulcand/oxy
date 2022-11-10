package trace

import "github.com/vulcand/oxy/v2/utils"

// Option is a functional option setter for Tracer.
type Option func(*Tracer) error

// ErrorHandler is a functional argument that sets error handler of the server.
func ErrorHandler(h utils.ErrorHandler) Option {
	return func(t *Tracer) error {
		t.errHandler = h
		return nil
	}
}

// RequestHeaders adds request headers to capture.
func RequestHeaders(headers ...string) Option {
	return func(t *Tracer) error {
		t.reqHeaders = append(t.reqHeaders, headers...)
		return nil
	}
}

// ResponseHeaders adds response headers to capture.
func ResponseHeaders(headers ...string) Option {
	return func(t *Tracer) error {
		t.respHeaders = append(t.respHeaders, headers...)
		return nil
	}
}

// Logger defines the logger the tracer will use.
func Logger(l utils.Logger) Option {
	return func(t *Tracer) error {
		t.log = l
		return nil
	}
}
