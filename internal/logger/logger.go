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

func Info(info string, args ...interface{}) {
	log.Info(info,
		args,
	)
}

func Warn(info string, args ...interface{}) {
	log.Warn(
		info,
		args,
	)
}
