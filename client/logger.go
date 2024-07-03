package client

import (
	"io"
	"strings"

	"go.uber.org/zap"
)

// yamuxLogWriter is an adapter to log yamux logs with the client logger.
type yamuxLogWriter struct {
	logger Logger
}

func (l *yamuxLogWriter) Write(b []byte) (int, error) {
	s := string(b)
	s = strings.TrimSpace(s)

	_, after, found := strings.Cut(s, "[WARN] ")
	if found {
		l.logger.Warn(after)
		return len(b), nil
	}
	_, after, found = strings.Cut(s, "[ERR] ")
	if found {
		l.logger.Error(after)
		return len(b), nil
	}

	_, after, found = strings.Cut(s, "yamux: ")
	if found {
		l.logger.Error("yamux: " + after)
		return len(b), nil
	}

	// Log unexpected formats as errors.
	l.logger.Error(s)
	return len(b), nil
}

var _ io.Writer = &yamuxLogWriter{}

// Logger is a logger compatible with zap.Logger.
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Sync() error
}
