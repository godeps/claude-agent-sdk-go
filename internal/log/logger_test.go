package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLogger_Verbose(t *testing.T) {
	logger := NewLogger(true)

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.slog == nil {
		t.Fatal("expected non-nil slog logger")
	}
	if !logger.verbose {
		t.Error("expected verbose=true")
	}

	// Verify debug messages are logged when verbose
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger.slog = slog.New(handler)
	logger.Debug("test debug %s", "msg")

	if !strings.Contains(buf.String(), "test debug msg") {
		t.Errorf("expected debug message in output, got: %s", buf.String())
	}
}

func TestNewLogger_NonVerbose(t *testing.T) {
	logger := NewLogger(false)

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.verbose {
		t.Error("expected verbose=false")
	}

	// Verify debug messages are NOT logged when non-verbose
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger.slog = slog.New(handler)
	logger.Debug("should not appear")

	if strings.Contains(buf.String(), "should not appear") {
		t.Errorf("debug message should not appear in non-verbose mode, got: %s", buf.String())
	}

	// Verify warn messages ARE logged
	logger.Warning("visible warning")
	if !strings.Contains(buf.String(), "visible warning") {
		t.Errorf("expected warning message in output, got: %s", buf.String())
	}
}

func TestNewLoggerFromSlog_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	customLogger := slog.New(handler)

	logger := NewLoggerFromSlog(customLogger)

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if !logger.verbose {
		t.Error("expected verbose=true when providing custom logger")
	}

	logger.Info("hello from custom")
	if !strings.Contains(buf.String(), "hello from custom") {
		t.Errorf("expected message in buffer, got: %s", buf.String())
	}
}

func TestNewLoggerFromSlog_Nil(t *testing.T) {
	// Should not panic, falls back to default
	logger := NewLoggerFromSlog(nil)

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.slog == nil {
		t.Fatal("expected non-nil slog logger")
	}
	if logger.verbose {
		t.Error("expected verbose=false for nil fallback")
	}
}

func TestNewLoggerWithLevel(t *testing.T) {
	tests := []struct {
		name            string
		level           slog.Level
		expectVerbose   bool
	}{
		{"debug_level", slog.LevelDebug, true},
		{"info_level", slog.LevelInfo, false},
		{"warn_level", slog.LevelWarn, false},
		{"error_level", slog.LevelError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLoggerWithLevel(tt.level)
			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
			if logger.verbose != tt.expectVerbose {
				t.Errorf("expected verbose=%v for level %v, got %v", tt.expectVerbose, tt.level, logger.verbose)
			}
		})
	}
}

func TestLogger_Slog(t *testing.T) {
	logger := NewLogger(false)

	slogLogger := logger.Slog()
	if slogLogger == nil {
		t.Fatal("expected non-nil *slog.Logger from Slog()")
	}
}
