package utils

// Logger the logger interface.
type Logger interface {
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Warnf(string, ...interface{})
	Errorf(string, ...interface{})
	Fatalf(string, ...interface{})
}

// NoopLogger a noop logger.
type NoopLogger struct{}

// Debugf noop.
func (*NoopLogger) Debugf(string, ...interface{}) {}

// Infof noop.
func (*NoopLogger) Infof(string, ...interface{}) {}

// Warnf noop.
func (*NoopLogger) Warnf(string, ...interface{}) {}

// Errorf noop.
func (*NoopLogger) Errorf(string, ...interface{}) {}

// Fatalf noop.
func (*NoopLogger) Fatalf(string, ...interface{}) {}
