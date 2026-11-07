package util

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var levelVar = new(slog.LevelVar)

var logger = newLogger()

func newLogger() *slog.Logger {
	levelVar.Set(slog.LevelInfo)
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: levelVar}))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}

// SetLogLevel changes the minimum severity at runtime ("debug", "info", "warn", "error").
func SetLogLevel(level string) { levelVar.Set(parseLevel(level)) }

func logf(level slog.Level, format string, args ...any) {
	if !logger.Enabled(context.Background(), level) {
		return
	}
	logger.Log(context.Background(), level, fmt.Sprintf(format, args...))
}

// LogDebug logs a formatted message at DEBUG severity.
func LogDebug(format string, args ...any) { logf(slog.LevelDebug, format, args...) }

// LogInfo logs a formatted message at INFO severity.
func LogInfo(format string, args ...any) { logf(slog.LevelInfo, format, args...) }

// LogWarn logs a formatted message at WARN severity.
func LogWarn(format string, args ...any) { logf(slog.LevelWarn, format, args...) }

// LogError logs a formatted message at ERROR severity.
func LogError(format string, args ...any) { logf(slog.LevelError, format, args...) }
