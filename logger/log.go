package logger

import (
	"os"

	"github.com/Sirupsen/logrus"
)

type Log interface {
	WithField(key string, value interface{}) Log
	WithError(err error) Log
	Debug(string, ...interface{})
	Error(string, ...interface{})
	Fatal(string, ...interface{})
	Info(string, ...interface{})
	Warning(string, ...interface{})
}

var _ Log = &logrusBridge{}

type logrusBridge struct {
	log *logrus.Entry
}

func (lb *logrusBridge) WithError(err error) Log {
	return &logrusBridge{
		log: lb.log.WithError(err),
	}
}
func (lb *logrusBridge) Debug(s string, opts ...interface{}) {
	lb.log.Debugf(s, opts...)
}

func (lb *logrusBridge) Error(s string, opts ...interface{}) {
	lb.log.Errorf(s, opts...)
}

func (lb *logrusBridge) Fatal(s string, opts ...interface{}) {
	lb.log.Fatalf(s, opts...)
}

func (lb *logrusBridge) Warning(s string, opts ...interface{}) {
	lb.log.Warningf(s, opts...)
}

func (lb *logrusBridge) Info(s string, opts ...interface{}) {
	lb.log.Infof(s, opts...)
}

func (lb *logrusBridge) WithField(key string, value interface{}) Log {
	return &logrusBridge{
		log: lb.log.WithField(key, value),
	}
}

func DefaultLogger() Log {
	ll := logrus.New()
	if os.Getenv("DEBUG") != "" {
		ll.Level = logrus.DebugLevel
	}

	return &logrusBridge{
		log: ll.WithFields(logrus.Fields{}),
	}
}
