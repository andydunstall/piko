package log

import (
	"bytes"
	"fmt"
	stdlog "log"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type loggerWriter struct {
	logFunc func(msg string, fields ...zap.Field)
}

func (l *loggerWriter) Write(p []byte) (int, error) {
	p = bytes.TrimSpace(p)
	l.logFunc(string(p))
	return len(p), nil
}

// Logger is a logger which writes structured logs to stderr formatted
// as JSON.
//
// Logs can be filtered by level, where only logs whose level exceeds the
// configured minimum level are logged. The log level can be overridden to
// include all logs matching the enabled subsystems.
//
// Logger is a simplified zap.Logger (which uses zapcore). zap.Logger had to
// be reimplemented to support overriding the log level filter.
type Logger interface {
	Subsystem() string
	// WithSubsystem creates a new logger with the given subsystem.
	WithSubsystem(s string) Logger
	With(fields ...zap.Field) Logger
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Sync() error
	// StdLogger returns a standard library log.Logger that logs records using
	// with the given level.
	StdLogger(level zapcore.Level) *stdlog.Logger
}

type logger struct {
	core zapcore.Core

	subsystem         string
	subsystemEnabled  bool
	enabledSubsystems []string

	errorOutput zapcore.WriteSyncer
}

// NewLogger creates a new logger filtering using the given log level and
// enabled subsystems.
func NewLogger(lvl string, enabledSubsystems []string) (Logger, error) {
	zapLevel, err := zapLevelFromString(lvl)
	if err != nil {
		return nil, err
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	// Using the logger name for 'subsystem'.
	encoderConfig.NameKey = "subsystem"
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(
		"2006-01-02T15:04:05.999Z07:00",
	)

	enc := zapcore.NewJSONEncoder(encoderConfig)
	sink, _, err := zap.Open("stderr")
	if err != nil {
		return nil, fmt.Errorf("open sync: %w", err)
	}
	core := &core{core: zapcore.NewCore(
		enc, sink, zap.NewAtomicLevelAt(zapLevel),
	)}
	return &logger{
		core: core,
		// Use 'main' as default subsystem.
		subsystem:         "main",
		subsystemEnabled:  subsystemMatch("main", enabledSubsystems),
		enabledSubsystems: enabledSubsystems,
		errorOutput:       zapcore.Lock(os.Stderr),
	}, nil
}

func (l *logger) Subsystem() string {
	return l.subsystem
}

func (l *logger) WithSubsystem(s string) Logger {
	if s == l.subsystem {
		return l
	}

	clone := l.clone()
	clone.subsystem = s
	clone.subsystemEnabled = subsystemMatch(s, clone.enabledSubsystems)
	return clone
}

// With creates a new logger with the given fields.
func (l *logger) With(fields ...zap.Field) Logger {
	if len(fields) == 0 {
		return l
	}
	clone := l.clone()
	clone.core = clone.core.With(fields)
	return clone
}

func (l *logger) Debug(msg string, fields ...zap.Field) {
	if ce := l.check(zap.DebugLevel, msg); ce != nil {
		ce.Write(fields...)
	}
}

func (l *logger) Info(msg string, fields ...zap.Field) {
	if ce := l.check(zap.InfoLevel, msg); ce != nil {
		ce.Write(fields...)
	}
}

func (l *logger) Warn(msg string, fields ...zap.Field) {
	if ce := l.check(zap.WarnLevel, msg); ce != nil {
		ce.Write(fields...)
	}
}

func (l *logger) Error(msg string, fields ...zap.Field) {
	if ce := l.check(zap.ErrorLevel, msg); ce != nil {
		ce.Write(fields...)
	}
}

func (l *logger) Sync() error {
	return l.core.Sync()
}

func (l *logger) StdLogger(level zapcore.Level) *stdlog.Logger {
	return stdlog.New(&loggerWriter{
		logFunc: func(msg string, fields ...zap.Field) {
			if ce := l.check(level, msg); ce != nil {
				ce.Write(fields...)
			}
		},
	}, "", 0)
}

func (l *logger) clone() *logger {
	clone := *l
	return &clone
}

func (l *logger) check(lvl zapcore.Level, msg string) *zapcore.CheckedEntry {
	// Only filter by log level if the subsystem isn't enabled.
	if !l.subsystemEnabled {
		if lvl < zapcore.DPanicLevel && !l.core.Enabled(lvl) {
			return nil
		}
	}

	ent := zapcore.Entry{
		// Use the logger name for subsystem. This is configured above to log
		// as a 'subsystem' field.
		LoggerName: l.subsystem,
		Time:       time.Now(),
		Level:      lvl,
		Message:    msg,
	}
	ce := l.core.Check(ent, nil)
	if ce == nil {
		return ce
	}

	// Thread the error output through to the CheckedEntry.
	ce.ErrorOutput = l.errorOutput

	return ce
}

type nopLogger struct {
}

func NewNopLogger() Logger {
	return &nopLogger{}
}

func (l *nopLogger) Subsystem() string {
	return ""
}

func (l *nopLogger) WithSubsystem(_ string) Logger {
	return l
}

func (l *nopLogger) With(_ ...zap.Field) Logger {
	return l
}

func (l *nopLogger) Debug(_ string, _ ...zap.Field) {
}

func (l *nopLogger) Info(_ string, _ ...zap.Field) {
}

func (l *nopLogger) Warn(_ string, _ ...zap.Field) {
}

func (l *nopLogger) Error(_ string, _ ...zap.Field) {
}

func (l *nopLogger) Sync() error {
	return nil
}

func (l *nopLogger) StdLogger(_ zapcore.Level) *stdlog.Logger {
	return stdlog.New(&loggerWriter{
		logFunc: func(msg string, fields ...zap.Field) {
		},
	}, "", 0)
}

func subsystemMatch(subsystem string, enabled []string) bool {
	for _, s := range enabled {
		if subsystem == s {
			return true
		}
	}
	return false
}

func zapLevelFromString(s string) (zapcore.Level, error) {
	switch s {
	case "debug":
		return zap.DebugLevel, nil
	case "info":
		return zap.InfoLevel, nil
	case "warn":
		return zap.WarnLevel, nil
	case "error":
		return zap.ErrorLevel, nil
	default:
		return zapcore.Level(0), fmt.Errorf("unsupported level: %s", s)
	}
}

// core is a wrapper for another core, except `Check()` will not filter by
// log level. This is required to log records matching the a configured
// subsystem.
type core struct {
	core zapcore.Core
}

func (c *core) Enabled(lvl zapcore.Level) bool {
	return c.core.Enabled(lvl)
}

func (c *core) With(fields []zap.Field) zapcore.Core {
	inner := c.core.With(fields)
	return &core{
		core: inner,
	}
}

func (c *core) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(ent, c.core)
}

func (c *core) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	return c.core.Write(ent, fields)
}

func (c *core) Sync() error {
	return c.core.Sync()
}
