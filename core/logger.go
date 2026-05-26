package core

import (
	"log/slog"
	"os"
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

	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	logger = slog.New(h)
}

// Logger returns the global structured logger.
// Returns a no-op logger if not initialized (safe for tests).
func Logger() *slog.Logger {
	if logger == nil {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	return logger
}
