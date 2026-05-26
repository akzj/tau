package core

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLoggerTextOutput(t *testing.T) {
	var buf bytes.Buffer
	logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	Info("test message", "key", "value")
	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected 'test message', got %q", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("expected key=value, got %q", output)
	}
}

func TestLoggerJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	Info("json test", "x", 1)
	output := buf.String()
	if !strings.Contains(output, `"msg":"json test"`) {
		t.Errorf("expected json msg, got %q", output)
	}
}

func TestLoggerLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	Info("should not appear")
	Warn("should appear")
	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Error("Info should be filtered out")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("Warn should appear")
	}
}

func TestLoggerWith(t *testing.T) {
	var buf bytes.Buffer
	logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	sub := With("component", "test")
	sub.Info("from sub-logger")
	output := buf.String()
	if !strings.Contains(output, "component=test") {
		t.Errorf("expected component=test, got %q", output)
	}
}

func TestLoggerStderrDefault(t *testing.T) {
	l := Logger()
	if l == nil {
		t.Error("Logger() should never return nil")
	}
}
