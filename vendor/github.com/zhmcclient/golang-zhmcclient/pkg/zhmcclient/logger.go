package zhmcclient

import (
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Debug(msg string, fields ...zapcore.Field)
	Info(msg string, fields ...zapcore.Field)
	Warn(msg string, fields ...zapcore.Field)
	Error(msg string, fields ...zapcore.Field)
	DPanic(msg string, fields ...zapcore.Field)
	Panic(msg string, fields ...zapcore.Field)
	Fatal(msg string, fields ...zapcore.Field)
}
