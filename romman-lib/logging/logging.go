// Package logging provides structured logging for ROM Manager.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Config holds logging configuration.
type Config struct {
	Format string // "json" or "text"
	Level  string // "debug", "info", "warn", "error"
}

// DefaultConfig returns sensible logging defaults.
func DefaultConfig() Config {
	return Config{
		Format: "text",
		Level:  "info",
	}
}

var logger *slog.Logger

// Setup initializes the global logger with the given configuration.
func Setup(cfg Config) {
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// parseLevel converts a string level to slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Get returns the configured logger, or the default if not set up.
func Get() *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}
