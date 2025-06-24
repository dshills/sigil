// Package logger provides structured logging for Sigil using slog.
package logger

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

var (
	// Default logger instance
	defaultLogger *slog.Logger
	// Global log level
	globalLevel = slog.LevelInfo
)

// Initialize sets up the default logger
func Initialize(level string, format string) {
	globalLevel = parseLevel(level)

	opts := &slog.HandlerOptions{
		Level: globalLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize time format
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, time.Now().Format("2006-01-02 15:04:05"))
			}
			// Add source location for debug level
			if globalLevel <= slog.LevelDebug && a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok {
					return slog.String("source", shortenPath(src.File)+":"+string(rune(src.Line)))
				}
			}
			return a
		},
	}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	default:
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// Get returns the default logger
func Get() *slog.Logger {
	if defaultLogger == nil {
		Initialize("info", "text")
	}
	return defaultLogger
}

// WithContext returns a logger with context values
func WithContext(ctx context.Context) *slog.Logger {
	logger := Get()

	// Extract common context values
	if reqID, ok := ctx.Value("request_id").(string); ok {
		logger = logger.With("request_id", reqID)
	}
	if userID, ok := ctx.Value("user_id").(string); ok {
		logger = logger.With("user_id", userID)
	}

	return logger
}

// WithOperation returns a logger for a specific operation
func WithOperation(op string) *slog.Logger {
	return Get().With("operation", op)
}

// WithError returns a logger with error context
func WithError(err error) *slog.Logger {
	return Get().With("error", err.Error())
}

// Helper functions for common log patterns

// Debug logs a debug message
func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, args ...any) {
	Get().Error(msg, args...)
	os.Exit(1)
}

// parseLevel converts string level to slog.Level
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

// shortenPath returns just the filename from a full path
func shortenPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// SetLevel changes the global log level
func SetLevel(level string) {
	globalLevel = parseLevel(level)
	Initialize(level, "text") // Re-initialize with new level
}

// IsDebugEnabled returns true if debug logging is enabled
func IsDebugEnabled() bool {
	return globalLevel <= slog.LevelDebug
}

// Trace logs a trace message (at debug level with TRACE prefix)
func Trace(msg string, args ...any) {
	if IsDebugEnabled() {
		_, file, line, _ := runtime.Caller(1)
		Get().Debug("[TRACE] "+msg, append(args, "source", shortenPath(file)+":"+string(rune(line)))...)
	}
}
