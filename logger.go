package pyongo

import "go.uber.org/zap/zapcore"

// Interface for wrapping zap loggers
type Logger interface {
	Info(msg string, fields ...zapcore.Field)
	Warn(msg string, fields ...zapcore.Field)
	Error(msg string, fields ...zapcore.Field)
}

type disabledLogger struct{}

func newDisabledLogger() *disabledLogger {
	return &disabledLogger{}
}

func (l *disabledLogger) Info(
	msg string,
	fields ...zapcore.Field,
) {
}

func (l *disabledLogger) Warn(
	msg string,
	fields ...zapcore.Field,
) {
}

func (l *disabledLogger) Error(
	msg string,
	fields ...zapcore.Field,
) {
}
