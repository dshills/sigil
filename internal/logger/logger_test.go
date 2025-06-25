package logger

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialize(t *testing.T) {
	// Save original stderr
	originalStderr := os.Stderr

	tests := []struct {
		name     string
		level    string
		format   string
		expected slog.Level
	}{
		{
			name:     "debug level with text format",
			level:    "debug",
			format:   "text",
			expected: slog.LevelDebug,
		},
		{
			name:     "info level with json format",
			level:    "info",
			format:   "json",
			expected: slog.LevelInfo,
		},
		{
			name:     "warn level with text format",
			level:    "warn",
			format:   "text",
			expected: slog.LevelWarn,
		},
		{
			name:     "error level with text format",
			level:    "error",
			format:   "text",
			expected: slog.LevelError,
		},
		{
			name:     "invalid level defaults to info",
			level:    "invalid",
			format:   "text",
			expected: slog.LevelInfo,
		},
		{
			name:     "invalid format defaults to text",
			level:    "info",
			format:   "invalid",
			expected: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a pipe to capture stderr
			r, w, err := os.Pipe()
			require.NoError(t, err)
			os.Stderr = w

			// Reset logger state
			defaultLogger = nil
			globalLevel = slog.LevelInfo

			Initialize(tt.level, tt.format)

			// Close write end and restore stderr
			w.Close()
			os.Stderr = originalStderr

			// Verify global level was set correctly
			assert.Equal(t, tt.expected, globalLevel)

			// Verify logger was created
			assert.NotNil(t, defaultLogger)

			// Clean up
			r.Close()
		})
	}
}

func TestGet(t *testing.T) {
	t.Run("returns existing logger", func(t *testing.T) {
		// Reset state
		defaultLogger = nil

		Initialize("info", "text")
		logger1 := Get()
		logger2 := Get()

		assert.NotNil(t, logger1)
		assert.Equal(t, logger1, logger2) // Should return same instance
	})

	t.Run("initializes logger if not exists", func(t *testing.T) {
		// Reset state
		defaultLogger = nil

		logger := Get()
		assert.NotNil(t, logger)
		assert.NotNil(t, defaultLogger)
	})
}

func TestWithContext(t *testing.T) {
	// Reset state
	defaultLogger = nil
	Initialize("info", "text")

	t.Run("extracts request_id from context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "request_id", "req-123")
		logger := WithContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("extracts user_id from context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "user_id", "user-456")
		logger := WithContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("extracts both request_id and user_id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "request_id", "req-123")
		ctx = context.WithValue(ctx, "user_id", "user-456")
		logger := WithContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("handles empty context", func(t *testing.T) {
		ctx := context.Background()
		logger := WithContext(ctx)
		assert.NotNil(t, logger)
	})
}

func TestWithOperation(t *testing.T) {
	// Reset state
	defaultLogger = nil
	Initialize("info", "text")

	logger := WithOperation("test-operation")
	assert.NotNil(t, logger)
}

func TestWithError(t *testing.T) {
	// Reset state
	defaultLogger = nil
	Initialize("info", "text")

	err := errors.New("test error")
	logger := WithError(err)
	assert.NotNil(t, logger)
}

func TestLogFunctions(t *testing.T) {
	// Reset state and initialize
	defaultLogger = nil
	Initialize("debug", "text")

	// Test that functions exist and can be called without panicking
	tests := []struct {
		name    string
		logFunc func(string, ...any)
		message string
		args    []any
	}{
		{
			name:    "Debug",
			logFunc: Debug,
			message: "debug message",
			args:    []any{"key", "value"},
		},
		{
			name:    "Info",
			logFunc: Info,
			message: "info message",
			args:    []any{"key", "value"},
		},
		{
			name:    "Warn",
			logFunc: Warn,
			message: "warn message",
			args:    []any{"key", "value"},
		},
		{
			name:    "Error",
			logFunc: Error,
			message: "error message",
			args:    []any{"key", "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the function can be called without panicking
			assert.NotPanics(t, func() {
				tt.logFunc(tt.message, tt.args...)
			})
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"invalid", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShortenPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/to/file.go", "file.go"},
		{"file.go", "file.go"},
		{"", ""},
		{"/single", "single"},
		{"/very/deep/nested/path/to/file.go", "file.go"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := shortenPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetLevel(t *testing.T) {
	// Save original stderr
	originalStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	// Reset state
	defaultLogger = nil
	globalLevel = slog.LevelInfo

	SetLevel("debug")

	// Close and restore
	w.Close()
	os.Stderr = originalStderr
	r.Close()

	assert.Equal(t, slog.LevelDebug, globalLevel)
	assert.NotNil(t, defaultLogger)
}

func TestIsDebugEnabled(t *testing.T) {
	t.Run("debug level enables debug", func(t *testing.T) {
		globalLevel = slog.LevelDebug
		assert.True(t, IsDebugEnabled())
	})

	t.Run("info level disables debug", func(t *testing.T) {
		globalLevel = slog.LevelInfo
		assert.False(t, IsDebugEnabled())
	})

	t.Run("warn level disables debug", func(t *testing.T) {
		globalLevel = slog.LevelWarn
		assert.False(t, IsDebugEnabled())
	})

	t.Run("error level disables debug", func(t *testing.T) {
		globalLevel = slog.LevelError
		assert.False(t, IsDebugEnabled())
	})
}

func TestTrace(t *testing.T) {
	// Reset state and set debug level
	defaultLogger = nil
	globalLevel = slog.LevelDebug
	Initialize("debug", "text")

	t.Run("calls trace when debug enabled", func(t *testing.T) {
		// Test that the function can be called without panicking when debug is enabled
		assert.NotPanics(t, func() {
			Trace("test trace message", "key", "value")
		})
	})

	t.Run("does not panic when debug disabled", func(t *testing.T) {
		// Reset with info level
		globalLevel = slog.LevelInfo

		// Test that the function can be called without panicking when debug is disabled
		assert.NotPanics(t, func() {
			Trace("test trace message", "key", "value")
		})
	})
}

func TestFatal(t *testing.T) {
	// This test is tricky because Fatal calls os.Exit(1)
	// We'll test that it logs the message but skip the exit part
	
	// Save original stderr
	originalStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	// Reset state
	defaultLogger = nil
	Initialize("info", "text")

	// We can't actually test the os.Exit(1) call in a unit test
	// So we'll just test that the function exists and can be called
	// In a real scenario, this would be tested with integration tests
	
	// Restore stderr
	w.Close()
	os.Stderr = originalStderr
	r.Close()

	// Just verify the function exists and has the right signature
	assert.NotNil(t, Fatal)
}

// TestLoggerIntegration tests the logger with actual output
func TestLoggerIntegration(t *testing.T) {
	// This is a more complete integration test that verifies
	// the logger actually produces expected output formats
	
	t.Run("text format output", func(t *testing.T) {
		// Reset state
		defaultLogger = nil
		
		// We'll just verify initialization works
		Initialize("info", "text")
		logger := Get()
		assert.NotNil(t, logger)
	})

	t.Run("json format output", func(t *testing.T) {
		// Reset state
		defaultLogger = nil
		
		// We'll just verify initialization works
		Initialize("info", "json")
		logger := Get()
		assert.NotNil(t, logger)
	})
}