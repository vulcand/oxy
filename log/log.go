package log

import (
	"io/ioutil"

	"github.com/Sirupsen/logrus"
)

var log *logrus.Logger

const DebugLevel = logrus.DebugLevel

func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

func Warningf(format string, args ...interface{}) {
	log.Warningf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

func WithField(key string, value interface{}) *logrus.Entry {
	return log.WithField(key, value)
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	return log.WithFields(fields)
}

func GetLevel() logrus.Level {
	return log.Level
}

func init() {
	log = logrus.New()
}

func Disable() {
	log.Out = ioutil.Discard
}
