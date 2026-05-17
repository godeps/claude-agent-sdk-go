package log

import (
	"fmt"
	"log/slog"
	"os"
)

// Logger wraps slog.Logger for SDK internal use.
type Logger struct {
	slog    *slog.Logger
	verbose bool
}

// NewLogger creates a logger from the Verbose flag (backward compat).
func NewLogger(verbose bool) *Logger {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	return &Logger{
		slog:    slog.New(handler).With("component", "claude-sdk"),
		verbose: verbose,
	}
}

// NewLoggerFromSlog wraps a caller-provided slog.Logger.
func NewLoggerFromSlog(logger *slog.Logger) *Logger {
	if logger == nil {
		return NewLogger(false)
	}
	return &Logger{
		slog:    logger.With("component", "claude-sdk"),
		verbose: true,
	}
}

// NewLoggerWithLevel creates a logger with a specific slog level.
func NewLoggerWithLevel(level slog.Level) *Logger {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	return &Logger{
		slog:    slog.New(handler).With("component", "claude-sdk"),
		verbose: level <= slog.LevelDebug,
	}
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(format string, args ...interface{}) {
	l.slog.Debug(fmt.Sprintf(format, args...))
}

// Info logs at INFO level.
func (l *Logger) Info(format string, args ...interface{}) {
	l.slog.Info(fmt.Sprintf(format, args...))
}

// Warning logs at WARN level.
func (l *Logger) Warning(format string, args ...interface{}) {
	l.slog.Warn(fmt.Sprintf(format, args...))
}

// Error logs at ERROR level.
func (l *Logger) Error(format string, args ...interface{}) {
	l.slog.Error(fmt.Sprintf(format, args...))
}

// Slog returns the underlying slog.Logger.
func (l *Logger) Slog() *slog.Logger {
	return l.slog
}
