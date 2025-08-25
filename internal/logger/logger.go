package logger

import "go.uber.org/zap"

var log *zap.SugaredLogger

func Init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	log = l.Sugar()
}

func Info(msg string, kv ...interface{}) {
	log.Infow(msg, kv...)
}

func Warn(msg string, kv ...interface{}) {
	log.Warnw(msg, kv...)
}
