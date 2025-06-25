package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		errType  ErrorType
		op       string
		message  string
		expected string
	}{
		{
			name:     "config error",
			errType:  ErrorTypeConfig,
			op:       "Load",
			message:  "failed to load config",
			expected: "[CONFIG] Load: failed to load config",
		},
		{
			name:     "model error",
			errType:  ErrorTypeModel,
			op:       "CreateModel",
			message:  "invalid model configuration",
			expected: "[MODEL] CreateModel: invalid model configuration",
		},
		{
			name:     "git error",
			errType:  ErrorTypeGit,
			op:       "Commit",
			message:  "nothing to commit",
			expected: "[GIT] Commit: nothing to commit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.errType, tt.op, tt.message)

			assert.Equal(t, tt.errType, err.Type)
			assert.Equal(t, tt.op, err.Op)
			assert.Equal(t, tt.message, err.Message)
			assert.Nil(t, err.Err)
			assert.NotNil(t, err.Context)
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestWrap(t *testing.T) {
	t.Run("wrap with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("underlying error")
		err := Wrap(underlyingErr, ErrorTypeFS, "ReadFile", "failed to read file")

		assert.Equal(t, ErrorTypeFS, err.Type)
		assert.Equal(t, "ReadFile", err.Op)
		assert.Equal(t, "failed to read file", err.Message)
		assert.Equal(t, underlyingErr, err.Err)
		assert.NotNil(t, err.Context)

		expected := "[FILESYSTEM] ReadFile: failed to read file - underlying error"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("wrap nil error returns nil", func(t *testing.T) {
		err := Wrap(nil, ErrorTypeFS, "ReadFile", "failed to read file")
		assert.Nil(t, err)
	})
}

func TestSigilError_Error(t *testing.T) {
	t.Run("error without underlying error", func(t *testing.T) {
		err := &SigilError{
			Type:    ErrorTypeValidation,
			Op:      "ValidateInput",
			Message: "invalid parameter",
		}

		expected := "[VALIDATION] ValidateInput: invalid parameter"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("error with underlying error", func(t *testing.T) {
		underlyingErr := errors.New("original error")
		err := &SigilError{
			Type:    ErrorTypeNetwork,
			Op:      "HTTPRequest",
			Message: "request failed",
			Err:     underlyingErr,
		}

		expected := "[NETWORK] HTTPRequest: request failed - original error"
		assert.Equal(t, expected, err.Error())
	})
}

func TestSigilError_Unwrap(t *testing.T) {
	t.Run("unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := errors.New("underlying error")
		err := &SigilError{Err: underlyingErr}

		assert.Equal(t, underlyingErr, err.Unwrap())
	})

	t.Run("unwrap returns nil when no underlying error", func(t *testing.T) {
		err := &SigilError{}
		assert.Nil(t, err.Unwrap())
	})
}

func TestSigilError_Is(t *testing.T) {
	t.Run("same error type returns true", func(t *testing.T) {
		err1 := New(ErrorTypeConfig, "Load", "config error")
		err2 := New(ErrorTypeConfig, "Save", "different config error")

		assert.True(t, err1.Is(err2))
		assert.True(t, err2.Is(err1))
	})

	t.Run("different error type returns false", func(t *testing.T) {
		err1 := New(ErrorTypeConfig, "Load", "config error")
		err2 := New(ErrorTypeModel, "Create", "model error")

		assert.False(t, err1.Is(err2))
		assert.False(t, err2.Is(err1))
	})

	t.Run("non-SigilError returns false", func(t *testing.T) {
		sigilErr := New(ErrorTypeConfig, "Load", "config error")
		standardErr := errors.New("standard error")

		assert.False(t, sigilErr.Is(standardErr))
	})
}

func TestSigilError_WithContext(t *testing.T) {
	t.Run("add context to error", func(t *testing.T) {
		err := New(ErrorTypeFS, "WriteFile", "failed to write")

		result := err.WithContext("file", "test.txt").WithContext("size", 1024)

		assert.Equal(t, err, result) // should return same instance
		assert.Equal(t, "test.txt", err.Context["file"])
		assert.Equal(t, 1024, err.Context["size"])
	})

	t.Run("initialize context if nil", func(t *testing.T) {
		err := &SigilError{
			Type:    ErrorTypeFS,
			Op:      "WriteFile",
			Message: "failed to write",
			Context: nil,
		}

		result := err.WithContext("file", "test.txt")

		assert.Equal(t, err, result)
		assert.NotNil(t, err.Context)
		assert.Equal(t, "test.txt", err.Context["file"])
	})
}

func TestErrorConstructors(t *testing.T) {
	t.Run("ConfigError", func(t *testing.T) {
		err := ConfigError("Load", "invalid config")

		assert.Equal(t, ErrorTypeConfig, err.Type)
		assert.Equal(t, "Load", err.Op)
		assert.Equal(t, "invalid config", err.Message)
		assert.Nil(t, err.Err)
	})

	t.Run("ModelError", func(t *testing.T) {
		err := ModelError("CreateModel", "model not found")

		assert.Equal(t, ErrorTypeModel, err.Type)
		assert.Equal(t, "CreateModel", err.Op)
		assert.Equal(t, "model not found", err.Message)
		assert.Nil(t, err.Err)
	})

	t.Run("GitError", func(t *testing.T) {
		err := GitError("Commit", "nothing to commit")

		assert.Equal(t, ErrorTypeGit, err.Type)
		assert.Equal(t, "Commit", err.Op)
		assert.Equal(t, "nothing to commit", err.Message)
		assert.Nil(t, err.Err)
	})

	t.Run("ValidationError", func(t *testing.T) {
		err := ValidationError("ValidateInput", "missing required field")

		assert.Equal(t, ErrorTypeValidation, err.Type)
		assert.Equal(t, "ValidateInput", err.Op)
		assert.Equal(t, "missing required field", err.Message)
		assert.Nil(t, err.Err)
	})
}

func TestIsNotFound(t *testing.T) {
	t.Run("nil error returns false", func(t *testing.T) {
		assert.False(t, IsNotFound(nil))
	})

	t.Run("file not found SigilError returns true", func(t *testing.T) {
		err := New(ErrorTypeFS, "ReadFile", "file not found")
		assert.True(t, IsNotFound(err))
	})

	t.Run("directory not found SigilError returns true", func(t *testing.T) {
		err := New(ErrorTypeFS, "ListDir", "directory not found")
		assert.True(t, IsNotFound(err))
	})

	t.Run("other FS error returns false", func(t *testing.T) {
		err := New(ErrorTypeFS, "WriteFile", "permission denied")
		assert.False(t, IsNotFound(err))
	})

	t.Run("non-FS SigilError returns false", func(t *testing.T) {
		err := New(ErrorTypeConfig, "Load", "file not found")
		assert.False(t, IsNotFound(err))
	})

	t.Run("sentinel ErrNotFound returns true", func(t *testing.T) {
		assert.True(t, IsNotFound(ErrNotFound))
	})

	t.Run("wrapped sentinel ErrNotFound returns true", func(t *testing.T) {
		wrappedErr := fmt.Errorf("wrapped: %w", ErrNotFound)
		assert.True(t, IsNotFound(wrappedErr))
	})

	t.Run("other standard error returns false", func(t *testing.T) {
		err := errors.New("some other error")
		assert.False(t, IsNotFound(err))
	})
}

func TestErrorTypes(t *testing.T) {
	// Test all error type constants
	expectedTypes := map[ErrorType]string{
		ErrorTypeConfig:     "CONFIG",
		ErrorTypeModel:      "MODEL",
		ErrorTypeGit:        "GIT",
		ErrorTypeFS:         "FILESYSTEM",
		ErrorTypeValidation: "VALIDATION",
		ErrorTypeNetwork:    "NETWORK",
		ErrorTypeInput:      "INPUT",
		ErrorTypeOutput:     "OUTPUT",
		ErrorTypeInternal:   "INTERNAL",
	}

	for errType, expected := range expectedTypes {
		t.Run(string(errType), func(t *testing.T) {
			assert.Equal(t, expected, string(errType))
		})
	}
}

func TestSentinelErrors(t *testing.T) {
	t.Run("sentinel errors are defined", func(t *testing.T) {
		assert.NotNil(t, ErrNotFound)
		assert.NotNil(t, ErrInvalidInput)
		assert.NotNil(t, ErrNotGitRepo)
		assert.NotNil(t, ErrModelNotFound)

		assert.Equal(t, "not found", ErrNotFound.Error())
		assert.Equal(t, "invalid input", ErrInvalidInput.Error())
		assert.Equal(t, "not a git repository", ErrNotGitRepo.Error())
		assert.Equal(t, "model not found", ErrModelNotFound.Error())
	})
}

func TestErrorsAs(t *testing.T) {
	t.Run("errors.As works with SigilError", func(t *testing.T) {
		originalErr := New(ErrorTypeModel, "CreateModel", "failed to create model")
		wrappedErr := fmt.Errorf("wrapped: %w", originalErr)

		var sigilErr *SigilError
		require.True(t, errors.As(wrappedErr, &sigilErr))
		assert.Equal(t, ErrorTypeModel, sigilErr.Type)
		assert.Equal(t, "CreateModel", sigilErr.Op)
		assert.Equal(t, "failed to create model", sigilErr.Message)
	})
}

func TestErrorsIs(t *testing.T) {
	t.Run("errors.Is works with SigilError", func(t *testing.T) {
		err1 := New(ErrorTypeConfig, "Load", "config error")
		err2 := New(ErrorTypeConfig, "Save", "different operation")
		err3 := New(ErrorTypeModel, "Create", "model error")

		assert.True(t, errors.Is(err1, err2))
		assert.False(t, errors.Is(err1, err3))
	})

	t.Run("errors.Is works with wrapped SigilError", func(t *testing.T) {
		sigilErr := New(ErrorTypeConfig, "Load", "config error")
		wrappedErr := fmt.Errorf("wrapped: %w", sigilErr)
		targetErr := New(ErrorTypeConfig, "Save", "different operation")

		assert.True(t, errors.Is(wrappedErr, targetErr))
	})
}
