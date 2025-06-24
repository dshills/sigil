// Package errors provides a comprehensive error handling framework for Sigil.
package errors

import (
	"errors"
	"fmt"
)

// ErrorType represents the category of error
type ErrorType string

const (
	// Configuration errors
	ErrorTypeConfig ErrorType = "CONFIG"
	// Model/LLM related errors
	ErrorTypeModel ErrorType = "MODEL"
	// Git operation errors
	ErrorTypeGit ErrorType = "GIT"
	// File system errors
	ErrorTypeFS ErrorType = "FILESYSTEM"
	// Validation errors
	ErrorTypeValidation ErrorType = "VALIDATION"
	// Network/API errors
	ErrorTypeNetwork ErrorType = "NETWORK"
	// User input errors
	ErrorTypeInput ErrorType = "INPUT"
	// Output errors
	ErrorTypeOutput ErrorType = "OUTPUT"
	// Internal/unexpected errors
	ErrorTypeInternal ErrorType = "INTERNAL"
)

// SigilError represents a structured error with context
type SigilError struct {
	Type    ErrorType
	Op      string // Operation that failed
	Message string
	Err     error // Underlying error
	Context map[string]interface{}
}

// Error implements the error interface
func (e *SigilError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %s - %v", e.Type, e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Type, e.Op, e.Message)
}

// Unwrap allows error unwrapping
func (e *SigilError) Unwrap() error {
	return e.Err
}

// Is allows error comparison
func (e *SigilError) Is(target error) bool {
	t, ok := target.(*SigilError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// New creates a new SigilError
func New(errType ErrorType, op string, message string) *SigilError {
	return &SigilError{
		Type:    errType,
		Op:      op,
		Message: message,
		Context: make(map[string]interface{}),
	}
}

// Wrap wraps an existing error with SigilError context
func Wrap(err error, errType ErrorType, op string, message string) *SigilError {
	if err == nil {
		return nil
	}
	return &SigilError{
		Type:    errType,
		Op:      op,
		Message: message,
		Err:     err,
		Context: make(map[string]interface{}),
	}
}

// WithContext adds context to the error
func (e *SigilError) WithContext(key string, value interface{}) *SigilError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// Common error constructors

// ConfigError creates a configuration error
func ConfigError(op string, message string) *SigilError {
	return New(ErrorTypeConfig, op, message)
}

// ModelError creates a model/LLM error
func ModelError(op string, message string) *SigilError {
	return New(ErrorTypeModel, op, message)
}

// GitError creates a git operation error
func GitError(op string, message string) *SigilError {
	return New(ErrorTypeGit, op, message)
}

// ValidationError creates a validation error
func ValidationError(op string, message string) *SigilError {
	return New(ErrorTypeValidation, op, message)
}

// IsNotFound checks if an error indicates something was not found
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var sigilErr *SigilError
	if errors.As(err, &sigilErr) {
		return sigilErr.Type == ErrorTypeFS &&
			(sigilErr.Message == "file not found" || sigilErr.Message == "directory not found")
	}
	return errors.Is(err, ErrNotFound)
}

// Sentinel errors
var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrNotGitRepo    = errors.New("not a git repository")
	ErrModelNotFound = errors.New("model not found")
)
