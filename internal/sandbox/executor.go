// Package sandbox provides safe code execution and validation
package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/git"
	"github.com/dshills/sigil/internal/logger"
)

// Executor provides safe code execution in isolated environments
type Executor struct {
	worktreeManager *WorktreeManager
	validator       *Validator
	config          ExecutorConfig
}

// ExecutorConfig holds configuration for sandbox execution
type ExecutorConfig struct {
	Timeout         time.Duration `yaml:"timeout"`
	MaxWorktrees    int           `yaml:"max_worktrees"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	AllowedCommands []string      `yaml:"allowed_commands"`
	BlockedCommands []string      `yaml:"blocked_commands"`
	WorkingDir      string        `yaml:"working_dir"`
}

// DefaultExecutorConfig returns default configuration
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		Timeout:         5 * time.Minute,
		MaxWorktrees:    10,
		CleanupInterval: 1 * time.Hour,
		AllowedCommands: []string{
			"go", "npm", "node", "python", "python3", "pip", "pip3",
			"cargo", "rustc", "mvn", "gradle", "make", "cmake",
			"git", "ls", "cat", "grep", "find", "head", "tail",
			"echo", "wc", "sort", "uniq", "sed", "awk",
		},
		BlockedCommands: []string{
			"rm", "rmdir", "del", "delete", "format", "fdisk",
			"sudo", "su", "chmod", "chown", "passwd",
			"curl", "wget", "ssh", "scp", "rsync", "nc", "netcat",
			"systemctl", "service", "killall", "pkill",
		},
		WorkingDir: ".sigil/sandbox",
	}
}

// NewExecutor creates a new sandbox executor
func NewExecutor(repo *git.Repository, config ExecutorConfig) (*Executor, error) {
	worktreeManager, err := NewWorktreeManager(repo)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeConfig, "NewExecutor", "failed to create worktree manager")
	}

	validator, err := NewValidator()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeConfig, "NewExecutor", "failed to create validator")
	}

	executor := &Executor{
		worktreeManager: worktreeManager,
		validator:       validator,
		config:          config,
	}

	// Start cleanup routine
	go executor.cleanupRoutine()

	logger.Info("initialized sandbox executor", "timeout", config.Timeout, "max_worktrees", config.MaxWorktrees)
	return executor, nil
}

// ExecuteCode executes code in a sandbox environment
func (e *Executor) ExecuteCode(ctx context.Context, request ExecutionRequest) (*ExecutionResponse, error) {
	logger.Debug("executing code in sandbox", "type", request.Type, "files", len(request.Files))

	// Validate the request
	if err := e.validator.ValidateRequest(request); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeValidation, "ExecuteCode", "request validation failed")
	}

	// Create a worktree
	worktree, err := e.worktreeManager.CreateWorktree("HEAD")
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeGit, "ExecuteCode", "failed to create worktree")
	}

	response := &ExecutionResponse{
		RequestID:  request.ID,
		WorktreeID: worktree.ID,
		StartTime:  time.Now(),
		Status:     StatusRunning,
	}

	// Ensure cleanup
	defer func() {
		if err := worktree.Cleanup(); err != nil {
			logger.Warn("failed to cleanup worktree", "id", worktree.ID, "error", err)
		}
	}()

	// Apply changes to worktree
	if err := e.applyChanges(worktree, request); err != nil {
		response.Status = StatusFailed
		response.Error = err.Error()
		response.EndTime = time.Now()
		return response, errors.Wrap(err, errors.ErrorTypeFS, "ExecuteCode", "failed to apply changes")
	}

	// Execute the validation steps
	if err := e.executeValidation(ctx, worktree, request, response); err != nil {
		response.Status = StatusFailed
		response.Error = err.Error()
		response.EndTime = time.Now()
		return response, err
	}

	response.Status = StatusCompleted
	response.EndTime = time.Now()

	logger.Info("sandbox execution completed", "request_id", request.ID, "duration", response.EndTime.Sub(response.StartTime))
	return response, nil
}

// applyChanges applies the requested changes to the worktree
func (e *Executor) applyChanges(worktree *Worktree, request ExecutionRequest) error {
	for _, file := range request.Files {
		logger.Debug("applying file change", "path", file.Path, "operation", file.Operation)

		switch file.Operation {
		case OperationCreate, OperationUpdate:
			if err := worktree.WriteFile(file.Path, []byte(file.Content)); err != nil {
				return errors.Wrap(err, errors.ErrorTypeFS, "applyChanges",
					fmt.Sprintf("failed to write file %s", file.Path))
			}

		case OperationDelete:
			fullPath := filepath.Join(worktree.Path, file.Path)
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				return errors.Wrap(err, errors.ErrorTypeFS, "applyChanges",
					fmt.Sprintf("failed to delete file %s", file.Path))
			}

		default:
			return errors.New(errors.ErrorTypeInput, "applyChanges",
				fmt.Sprintf("unknown file operation: %s", file.Operation))
		}
	}

	return nil
}

// executeValidation executes validation steps in the worktree
func (e *Executor) executeValidation(ctx context.Context, worktree *Worktree, request ExecutionRequest, response *ExecutionResponse) error {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Execute validation commands
	for _, step := range request.ValidationSteps {
		logger.Debug("executing validation step", "command", step.Command, "args", step.Args)

		// Validate command is allowed
		if !e.isCommandAllowed(step.Command) {
			return errors.New(errors.ErrorTypeValidation, "executeValidation",
				fmt.Sprintf("command not allowed: %s", step.Command))
		}

		// Execute the command
		result, err := e.executeCommand(execCtx, worktree, step)
		if err != nil {
			return errors.Wrap(err, errors.ErrorTypeInternal, "executeValidation", "command execution failed")
		}

		response.Results = append(response.Results, *result)

		// Check if validation step failed and if it's required to pass
		if !result.Success() && step.Required {
			return errors.New(errors.ErrorTypeValidation, "executeValidation",
				fmt.Sprintf("required validation step failed: %s", step.Command))
		}

		// Check for context cancellation
		select {
		case <-execCtx.Done():
			return errors.New(errors.ErrorTypeInternal, "executeValidation", "execution timeout")
		default:
			// Continue
		}
	}

	// Get final diff
	diff, err := worktree.GetChanges()
	if err != nil {
		logger.Warn("failed to get final diff", "error", err)
	} else {
		response.Diff = diff
	}

	return nil
}

// executeCommand executes a single command in the worktree
func (e *Executor) executeCommand(ctx context.Context, worktree *Worktree, step ValidationStep) (*ExecutionResult, error) {
	// Set up command with timeout monitoring
	done := make(chan *ExecutionResult, 1)
	errChan := make(chan error, 1)

	go func() {
		result, err := worktree.Execute(step.Command, step.Args...)
		if err != nil {
			errChan <- err
			return
		}
		done <- result
	}()

	select {
	case result := <-done:
		return result, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, errors.New(errors.ErrorTypeInternal, "executeCommand", "command execution timeout")
	}
}

// isCommandAllowed checks if a command is allowed to execute
func (e *Executor) isCommandAllowed(command string) bool {
	// Check if command is explicitly blocked
	for _, blocked := range e.config.BlockedCommands {
		if command == blocked {
			return false
		}
	}

	// Check if command is in allowed list
	for _, allowed := range e.config.AllowedCommands {
		if command == allowed {
			return true
		}
	}

	// Default deny
	return false
}

// cleanupRoutine periodically cleans up old worktrees
func (e *Executor) cleanupRoutine() {
	ticker := time.NewTicker(e.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		logger.Debug("running worktree cleanup")
		if err := e.worktreeManager.CleanupOldWorktrees(e.config.CleanupInterval * 2); err != nil {
			logger.Warn("worktree cleanup failed", "error", err)
		}
	}
}

// GetWorktrees returns all active worktrees
func (e *Executor) GetWorktrees() []*Worktree {
	return e.worktreeManager.ListWorktrees()
}

// Cleanup cleans up all resources
func (e *Executor) Cleanup() error {
	logger.Info("cleaning up sandbox executor")

	// Cleanup all worktrees
	for _, worktree := range e.worktreeManager.ListWorktrees() {
		if err := worktree.Cleanup(); err != nil {
			logger.Warn("failed to cleanup worktree during shutdown", "id", worktree.ID, "error", err)
		}
	}

	return nil
}

// Execution types and constants

// ExecutionStatus represents the status of execution
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
	StatusTimeout   ExecutionStatus = "timeout"
)

// FileOperation represents a file operation
type FileOperation string

const (
	OperationCreate FileOperation = "create"
	OperationUpdate FileOperation = "update"
	OperationDelete FileOperation = "delete"
)

// ExecutionRequest represents a request for code execution
type ExecutionRequest struct {
	ID              string            `json:"id"`
	Type            string            `json:"type"` // "validation", "test", "build", etc.
	Files           []FileChange      `json:"files"`
	ValidationSteps []ValidationStep  `json:"validation_steps"`
	Context         map[string]string `json:"context,omitempty"`
}

// FileChange represents a change to a file
type FileChange struct {
	Path      string        `json:"path"`
	Content   string        `json:"content"`
	Operation FileOperation `json:"operation"`
}

// ValidationStep represents a validation command to execute
type ValidationStep struct {
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
}

// ExecutionResponse represents the response from code execution
type ExecutionResponse struct {
	RequestID  string            `json:"request_id"`
	WorktreeID string            `json:"worktree_id"`
	Status     ExecutionStatus   `json:"status"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	Results    []ExecutionResult `json:"results"`
	Diff       string            `json:"diff,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// Duration returns the execution duration
func (er *ExecutionResponse) Duration() time.Duration {
	if er.EndTime.IsZero() {
		return time.Since(er.StartTime)
	}
	return er.EndTime.Sub(er.StartTime)
}

// Success returns true if all required steps passed
func (er *ExecutionResponse) Success() bool {
	return er.Status == StatusCompleted
}
