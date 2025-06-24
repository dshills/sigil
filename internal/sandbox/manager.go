// Package sandbox provides sandbox management functionality
package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/git"
	"github.com/dshills/sigil/internal/logger"
	"gopkg.in/yaml.v3"
)

// DefaultManager implements the Manager interface
type DefaultManager struct {
	executor     *Executor
	validator    *Validator
	config       ProjectConfiguration
	metrics      SandboxMetrics
	eventManager *EventManager
	mu           sync.RWMutex
}

// NewManager creates a new sandbox manager
func NewManager(repo *git.Repository) (Manager, error) {
	// Load project configuration
	config, err := loadProjectConfig()
	if err != nil {
		logger.Warn("failed to load project config, using defaults", "error", err)
		config = detectProjectConfig()
	}

	// Create executor
	executorConfig := ExecutorConfig{
		Timeout:         config.Build.Timeout,
		MaxWorktrees:    10,
		CleanupInterval: 1 * time.Hour,
		AllowedCommands: getAllowedCommands(config),
		BlockedCommands: getBlockedCommands(),
		WorkingDir:      ".sigil/sandbox",
	}

	executor, err := NewExecutor(repo, executorConfig)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeConfig, "NewManager", "failed to create executor")
	}

	// Create validator
	validator, err := NewValidator()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeConfig, "NewManager", "failed to create validator")
	}

	manager := &DefaultManager{
		executor:     executor,
		validator:    validator,
		config:       config,
		eventManager: NewEventManager(),
		metrics: SandboxMetrics{
			TotalSandboxes:  0,
			ActiveSandboxes: 0,
			TotalExecutions: 0,
			SuccessfulRuns:  0,
			FailedRuns:      0,
		},
	}

	logger.Info("initialized sandbox manager", "language", config.Language, "framework", config.Framework)
	return manager, nil
}

// ExecuteCode executes code in a sandbox environment
func (m *DefaultManager) ExecuteCode(ctx context.Context, request ExecutionRequest) (*ExecutionResponse, error) {
	m.mu.Lock()
	m.metrics.TotalExecutions++
	m.mu.Unlock()

	// Emit start event
	m.eventManager.Emit(SandboxEvent{
		Type:      EventExecutionStarted,
		SandboxID: request.ID,
		Timestamp: time.Now(),
		Data: map[string]string{
			"type":       request.Type,
			"file_count": fmt.Sprintf("%d", len(request.Files)),
		},
	})

	// Execute the code
	response, err := m.executor.ExecuteCode(ctx, request)

	// Update metrics
	m.mu.Lock()
	if err != nil || !response.Success() {
		m.metrics.FailedRuns++
	} else {
		m.metrics.SuccessfulRuns++
	}
	m.mu.Unlock()

	// Emit end event
	eventType := EventExecutionEnded
	eventData := map[string]string{
		"status":   string(response.Status),
		"duration": response.Duration().String(),
	}
	eventError := ""

	if err != nil {
		eventType = EventValidationFailed
		eventError = err.Error()
	}

	m.eventManager.Emit(SandboxEvent{
		Type:      eventType,
		SandboxID: request.ID,
		Timestamp: time.Now(),
		Data:      eventData,
		Error:     eventError,
	})

	return response, err
}

// ValidateCode validates code without execution
func (m *DefaultManager) ValidateCode(path string, content string) error {
	return m.validator.ValidateCode(path, content)
}

// GetValidationRules returns validation rules for a path
func (m *DefaultManager) GetValidationRules(path string) ([]FileRule, []ContentRule) {
	return m.validator.GetRulesForPath(path)
}

// CreateSandbox creates a sandbox for manual operations
func (m *DefaultManager) CreateSandbox() (Sandbox, error) {
	worktree, err := m.executor.worktreeManager.CreateWorktree("HEAD")
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeGit, "CreateSandbox", "failed to create worktree")
	}

	m.mu.Lock()
	m.metrics.TotalSandboxes++
	m.metrics.ActiveSandboxes++
	m.mu.Unlock()

	// Emit event
	m.eventManager.Emit(SandboxEvent{
		Type:      EventSandboxCreated,
		SandboxID: worktree.ID,
		Timestamp: time.Now(),
		Data: map[string]string{
			"path": worktree.Path,
		},
	})

	return &SandboxAdapter{worktree: worktree, manager: m}, nil
}

// ListSandboxes lists active sandboxes
func (m *DefaultManager) ListSandboxes() []SandboxInfo {
	worktrees := m.executor.GetWorktrees()
	sandboxes := make([]SandboxInfo, 0, len(worktrees))

	for _, wt := range worktrees {
		sandboxes = append(sandboxes, SandboxInfo{
			ID:        wt.ID,
			Path:      wt.Path,
			CreatedAt: wt.CreatedAt,
			LastUsed:  wt.LastUsed,
			Status:    SandboxStatusActive,
		})
	}

	return sandboxes
}

// Cleanup cleans up all resources
func (m *DefaultManager) Cleanup() error {
	logger.Info("cleaning up sandbox manager")

	if err := m.executor.Cleanup(); err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Cleanup", "failed to cleanup executor")
	}

	m.mu.Lock()
	m.metrics.TotalCleanups++
	m.metrics.LastCleanupTime = time.Now()
	m.metrics.ActiveSandboxes = 0
	m.mu.Unlock()

	return nil
}

// GetMetrics returns sandbox metrics
func (m *DefaultManager) GetMetrics() SandboxMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// GetConfig returns project configuration
func (m *DefaultManager) GetConfig() ProjectConfiguration {
	return m.config
}

// AddEventObserver adds an event observer
func (m *DefaultManager) AddEventObserver(observer Observer) {
	m.eventManager.AddObserver(observer)
}

// SandboxAdapter adapts Worktree to Sandbox interface
type SandboxAdapter struct {
	worktree *Worktree
	manager  *DefaultManager
}

// ID returns the sandbox ID
func (s *SandboxAdapter) ID() string {
	return s.worktree.ID
}

// Path returns the sandbox path
func (s *SandboxAdapter) Path() string {
	return s.worktree.Path
}

// WriteFile writes a file to the sandbox
func (s *SandboxAdapter) WriteFile(path string, content []byte) error {
	return s.worktree.WriteFile(path, content)
}

// ReadFile reads a file from the sandbox
func (s *SandboxAdapter) ReadFile(path string) ([]byte, error) {
	return s.worktree.ReadFile(path)
}

// Execute executes a command in the sandbox
func (s *SandboxAdapter) Execute(command string, args ...string) (*ExecutionResult, error) {
	return s.worktree.Execute(command, args...)
}

// GetChanges gets changes in the sandbox
func (s *SandboxAdapter) GetChanges() (string, error) {
	return s.worktree.GetChanges()
}

// Commit commits changes
func (s *SandboxAdapter) Commit(message string) error {
	return s.worktree.Commit(message)
}

// Cleanup cleans up the sandbox
func (s *SandboxAdapter) Cleanup() error {
	s.manager.mu.Lock()
	s.manager.metrics.ActiveSandboxes--
	s.manager.mu.Unlock()

	// Emit event
	s.manager.eventManager.Emit(SandboxEvent{
		Type:      EventSandboxCleaned,
		SandboxID: s.worktree.ID,
		Timestamp: time.Now(),
	})

	return s.worktree.Cleanup()
}

// Helper functions

// loadProjectConfig loads project configuration from file
func loadProjectConfig() (ProjectConfiguration, error) {
	configPath := filepath.Join(".sigil", "project.yml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return ProjectConfiguration{}, errors.New(errors.ErrorTypeInput, "loadProjectConfig", "config file not found")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ProjectConfiguration{}, errors.Wrap(err, errors.ErrorTypeFS, "loadProjectConfig", "failed to read config file")
	}

	var config ProjectConfiguration
	if err := yaml.Unmarshal(data, &config); err != nil {
		return ProjectConfiguration{}, errors.Wrap(err, errors.ErrorTypeInput, "loadProjectConfig", "failed to parse config file")
	}

	return config, nil
}

// detectProjectConfig detects project configuration based on files present
func detectProjectConfig() ProjectConfiguration {
	defaults := DefaultProjectConfigurations()

	// Check for Go project
	if fileExists("go.mod") || fileExists("main.go") {
		return defaults["go"]
	}

	// Check for Node.js project
	if fileExists("package.json") {
		return defaults["node"]
	}

	// Check for Python project
	if fileExists("requirements.txt") || fileExists("pyproject.toml") || fileExists("setup.py") {
		return defaults["python"]
	}

	// Default to Go project
	return defaults["go"]
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// getAllowedCommands returns allowed commands based on project configuration
func getAllowedCommands(config ProjectConfiguration) []string {
	base := []string{
		"git", "ls", "cat", "grep", "find", "head", "tail",
		"echo", "wc", "sort", "uniq", "sed", "awk", "pwd",
	}

	switch config.Language {
	case "go":
		return append(base, "go", "gofmt", "golangci-lint")
	case "javascript":
		return append(base, "node", "npm", "yarn", "npx")
	case "python":
		return append(base, "python", "python3", "pip", "pip3", "pytest", "flake8")
	default:
		return base
	}
}

// getBlockedCommands returns blocked commands for security
func getBlockedCommands() []string {
	return []string{
		"rm", "rmdir", "del", "delete", "format", "fdisk",
		"sudo", "su", "chmod", "chown", "passwd",
		"curl", "wget", "ssh", "scp", "rsync", "nc", "netcat",
		"systemctl", "service", "killall", "pkill",
		"dd", "mount", "umount", "mkfs",
	}
}
