package core

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"
)

var logger *slog.Logger

// InitLogger initializes the global structured logger.
// level: "debug", "info", "warn", "error"
// format: "text" or "json"
func InitLogger(level, format string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl, AddSource: true}
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	logger = slog.New(h)
}

// Logger returns the global structured logger.
// Returns an Error-level logger if not initialized (safe for tests).
func Logger() *slog.Logger {
	if logger == nil {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	return logger
}

// Convenience methods for direct logging with auto source location.

// Debug logs at debug level.
func Debug(msg string, args ...any) { logAt(slog.LevelDebug, msg, args...) }

// Info logs at info level.
func Info(msg string, args ...any) { logAt(slog.LevelInfo, msg, args...) }

// Warn logs at warn level.
func Warn(msg string, args ...any) { logAt(slog.LevelWarn, msg, args...) }

func logAt(level slog.Level, msg string, args ...any) {
	if !Logger().Enabled(context.Background(), level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = Logger().Handler().Handle(context.Background(), r)
}

// With returns a sub-logger with additional context.
func With(args ...any) *slog.Logger {
	return Logger().With(args...)
}
