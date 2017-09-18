package logger

import (
	"os"

	"github.com/Sirupsen/logrus"
)

type Log interface {
	WithField(key string, value interface{}) Log
	WithError(err error) Log
	Debug(...interface{})
	Debugf(string, ...interface{})
	Error(...interface{})
	Errorf(string, ...interface{})
	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Info(...interface{})
	Infof(string, ...interface{})
	Warning(...interface{})
	Warningf(string, ...interface{})
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

func (lb *logrusBridge) Debug(opts ...interface{}) {
	lb.log.Debug(opts...)
}

func (lb *logrusBridge) Debugf(s string, opts ...interface{}) {
	lb.log.Debugf(s, opts...)
}

func (lb *logrusBridge) Error(opts ...interface{}) {
	lb.log.Error(opts...)
}

func (lb *logrusBridge) Errorf(s string, opts ...interface{}) {
	lb.log.Errorf(s, opts...)
}

func (lb *logrusBridge) Fatal(opts ...interface{}) {
	lb.log.Fatal(opts...)
}

func (lb *logrusBridge) Fatalf(s string, opts ...interface{}) {
	lb.log.Fatalf(s, opts...)
}

func (lb *logrusBridge) Warning(opts ...interface{}) {
	lb.log.Warning(opts...)
}

func (lb *logrusBridge) Warningf(s string, opts ...interface{}) {
	lb.log.Warningf(s, opts...)
}

func (lb *logrusBridge) Info(opts ...interface{}) {
	lb.log.Info(opts...)
}

func (lb *logrusBridge) Infof(s string, opts ...interface{}) {
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
