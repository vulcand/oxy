package utils

// Logger is a minimal logger interface which satisfies oxy logging needs.
type Logger interface {
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Warnf(string, ...interface{})
	Errorf(string, ...interface{})
	Fatalf(string, ...interface{})
}

type DefaultLogger struct{}

func (*DefaultLogger) Debugf(string, ...interface{}) {}
func (*DefaultLogger) Infof(string, ...interface{})  {}
func (*DefaultLogger) Warnf(string, ...interface{})  {}
func (*DefaultLogger) Errorf(string, ...interface{}) {}
func (*DefaultLogger) Fatalf(string, ...interface{}) {}

// LoggerDebugFunc is a function that returns a boolean that tells whether or not oxy
// should generate debug call that can be costly.
//
// If the logger you use is logrus, the function you should pass as option should
// look like this:
//
// 		import (
// 			"github.com/sirupsen/logrus"
// 		)
//
// 		stdLogger := logrus.StandardLogger()
// 		stdLogger.SetLevel(logrus.DebugLevel)
//
// 		logrusLogger := stdLogger.WithField("lib", "vulcand/oxy")
// 		logrusDebugFunc := func() bool {
// 			return logrusLogger.Logger.Level >= logrus.DebugLevel
// 		}
//
// 		cbLogger := cbreaker.Logger(logrusLogger)
// 		cbDebug := cbreaker.Debug(logrusDebugFunc)
//
// 		cb, err := cbreaker.New(next, "NetworkErrorRatio() > 0.3", cbLogger, cbDebug)
//
// If the logger you use is zap:
//
// 		import (
// 			"github.com/vulcand/oxy/v2/cbreaker"
// 			"go.uber.org/zap/zap"
// 			"go.uber.org/zap/zapcore"
// 		)
//
// 		zapAtomLevel := zap.NewAtomicLevel()
// 		zapAtomLevel.SetLevel(zapcore.DebugLevel)
//
// 		zapEncoderCfg := zap.NewProductionEncoderConfig()
// 		zapEncoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
//
// 		zapCore := zapcore.NewCore(zapcore.NewConsoleEncoder(zapEncoderCfg), zapcore.Lock(os.Stdout), zapAtomLevel)
// 		zapLogger := zap.New(zapCore).With(zap.String("lib", "vulcand/oxy"))
//
// 		zapSugaredLogger := zapLogger.Sugar()
// 		zapDebug  := func() bool {
// 			return zapAtomLevel.Enabled(zapcore.DebugLevel)
// 		}
//
// 		cbLogger := cbreaker.Logger(zapSugaredLogger)
// 		cbDebug := cbreaker.Debug(zapDebug)
//
// 		cb, err := cbreaker.New(next, "NetworkErrorRatio() > 0.3", cbLogger, cbDebug)
//
type LoggerDebugFunc func() bool

// DefaultLoggerDebugFunc is the default LoggerDebugFunc which returns false.
func DefaultLoggerDebugFunc() bool {
	return false
}
