package utils

// Logger the logger interface.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// NoopLogger a noop logger.
type NoopLogger struct{}

// Debug noop.
func (*NoopLogger) Debug(string, ...interface{}) {}

// Info noop.
func (*NoopLogger) Info(string, ...interface{}) {}

// Warn noop.
func (*NoopLogger) Warn(string, ...interface{}) {}

// Error noop.
func (*NoopLogger) Error(string, ...interface{}) {}
