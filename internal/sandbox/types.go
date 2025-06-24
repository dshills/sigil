// Package sandbox provides types and interfaces for sandbox execution
package sandbox

import (
	"context"
	"time"
)

// Manager provides high-level sandbox operations
type Manager interface {
	// Execute code in a sandbox environment
	ExecuteCode(ctx context.Context, request ExecutionRequest) (*ExecutionResponse, error)

	// Validate code without execution
	ValidateCode(path string, content string) error

	// Get validation rules for a path
	GetValidationRules(path string) ([]FileRule, []ContentRule)

	// Create a sandbox for manual operations
	CreateSandbox() (Sandbox, error)

	// List active sandboxes
	ListSandboxes() []SandboxInfo

	// Cleanup resources
	Cleanup() error
}

// Sandbox provides low-level sandbox operations
type Sandbox interface {
	// Get sandbox ID
	ID() string

	// Get sandbox path
	Path() string

	// Write a file to the sandbox
	WriteFile(path string, content []byte) error

	// Read a file from the sandbox
	ReadFile(path string) ([]byte, error)

	// Execute a command in the sandbox
	Execute(command string, args ...string) (*ExecutionResult, error)

	// Get changes in the sandbox
	GetChanges() (string, error)

	// Commit changes
	Commit(message string) error

	// Cleanup the sandbox
	Cleanup() error
}

// SandboxInfo provides information about a sandbox
type SandboxInfo struct {
	ID        string        `json:"id"`
	Path      string        `json:"path"`
	CreatedAt time.Time     `json:"created_at"`
	LastUsed  time.Time     `json:"last_used"`
	Status    SandboxStatus `json:"status"`
}

// SandboxStatus represents the status of a sandbox
type SandboxStatus string

const (
	SandboxStatusActive  SandboxStatus = "active"
	SandboxStatusIdle    SandboxStatus = "idle"
	SandboxStatusCleaned SandboxStatus = "cleaned"
)

// ValidationConfig provides validation configuration
type ValidationConfig struct {
	Enabled      bool          `yaml:"enabled"`
	RulesFile    string        `yaml:"rules_file"`
	StrictMode   bool          `yaml:"strict_mode"`
	Timeout      time.Duration `yaml:"timeout"`
	MaxFileSize  int64         `yaml:"max_file_size"`
	MaxTotalSize int64         `yaml:"max_total_size"`
	MaxFiles     int           `yaml:"max_files"`
}

// DefaultValidationConfig returns default validation configuration
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		Enabled:      true,
		RulesFile:    ".sigil/rules.yml",
		StrictMode:   false,
		Timeout:      30 * time.Second,
		MaxFileSize:  1024 * 1024,      // 1MB
		MaxTotalSize: 10 * 1024 * 1024, // 10MB
		MaxFiles:     100,
	}
}

// TestConfiguration holds test execution configuration
type TestConfiguration struct {
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Directory   string            `yaml:"directory"`
	Environment map[string]string `yaml:"environment"`
	Timeout     time.Duration     `yaml:"timeout"`
}

// BuildConfiguration holds build execution configuration
type BuildConfiguration struct {
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Directory   string            `yaml:"directory"`
	Environment map[string]string `yaml:"environment"`
	Timeout     time.Duration     `yaml:"timeout"`
	Artifacts   []string          `yaml:"artifacts"`
}

// LintConfiguration holds linting configuration
type LintConfiguration struct {
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Directory   string            `yaml:"directory"`
	Environment map[string]string `yaml:"environment"`
	Timeout     time.Duration     `yaml:"timeout"`
	ConfigFile  string            `yaml:"config_file"`
}

// ProjectConfiguration holds project-specific sandbox configuration
type ProjectConfiguration struct {
	Name        string             `yaml:"name"`
	Language    string             `yaml:"language"`
	Framework   string             `yaml:"framework"`
	Test        TestConfiguration  `yaml:"test"`
	Build       BuildConfiguration `yaml:"build"`
	Lint        LintConfiguration  `yaml:"lint"`
	Validation  ValidationConfig   `yaml:"validation"`
	Environment map[string]string  `yaml:"environment"`
}

// DefaultProjectConfigurations returns default configurations for common project types
func DefaultProjectConfigurations() map[string]ProjectConfiguration {
	return map[string]ProjectConfiguration{
		"go": {
			Name:      "Go Project",
			Language:  "go",
			Framework: "",
			Test: TestConfiguration{
				Command:   "go",
				Args:      []string{"test", "./..."},
				Directory: ".",
				Timeout:   5 * time.Minute,
			},
			Build: BuildConfiguration{
				Command:   "go",
				Args:      []string{"build", "./..."},
				Directory: ".",
				Timeout:   5 * time.Minute,
			},
			Lint: LintConfiguration{
				Command:    "golangci-lint",
				Args:       []string{"run"},
				Directory:  ".",
				Timeout:    5 * time.Minute,
				ConfigFile: ".golangci.yml",
			},
			Validation: DefaultValidationConfig(),
		},
		"node": {
			Name:      "Node.js Project",
			Language:  "javascript",
			Framework: "node",
			Test: TestConfiguration{
				Command:   "npm",
				Args:      []string{"test"},
				Directory: ".",
				Timeout:   5 * time.Minute,
			},
			Build: BuildConfiguration{
				Command:   "npm",
				Args:      []string{"run", "build"},
				Directory: ".",
				Timeout:   5 * time.Minute,
			},
			Lint: LintConfiguration{
				Command:   "npm",
				Args:      []string{"run", "lint"},
				Directory: ".",
				Timeout:   2 * time.Minute,
			},
			Validation: DefaultValidationConfig(),
		},
		"python": {
			Name:      "Python Project",
			Language:  "python",
			Framework: "",
			Test: TestConfiguration{
				Command:   "python",
				Args:      []string{"-m", "pytest"},
				Directory: ".",
				Timeout:   10 * time.Minute,
			},
			Build: BuildConfiguration{
				Command:   "python",
				Args:      []string{"-m", "build"},
				Directory: ".",
				Timeout:   5 * time.Minute,
			},
			Lint: LintConfiguration{
				Command:   "python",
				Args:      []string{"-m", "flake8"},
				Directory: ".",
				Timeout:   2 * time.Minute,
			},
			Validation: DefaultValidationConfig(),
		},
	}
}

// SandboxMetrics holds metrics about sandbox usage
type SandboxMetrics struct {
	TotalSandboxes  int           `json:"total_sandboxes"`
	ActiveSandboxes int           `json:"active_sandboxes"`
	TotalExecutions int64         `json:"total_executions"`
	SuccessfulRuns  int64         `json:"successful_runs"`
	FailedRuns      int64         `json:"failed_runs"`
	AverageExecTime time.Duration `json:"average_exec_time"`
	TotalCleanups   int64         `json:"total_cleanups"`
	DiskUsage       int64         `json:"disk_usage_bytes"`
	LastCleanupTime time.Time     `json:"last_cleanup_time"`
}

// SandboxEvent represents events in sandbox lifecycle
type SandboxEvent struct {
	Type      EventType         `json:"type"`
	SandboxID string            `json:"sandbox_id"`
	Timestamp time.Time         `json:"timestamp"`
	Data      map[string]string `json:"data,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// EventType represents the type of sandbox event
type EventType string

const (
	EventSandboxCreated   EventType = "sandbox_created"
	EventSandboxCleaned   EventType = "sandbox_cleaned"
	EventExecutionStarted EventType = "execution_started"
	EventExecutionEnded   EventType = "execution_ended"
	EventValidationFailed EventType = "validation_failed"
	EventTimeoutReached   EventType = "timeout_reached"
)

// Observer defines the interface for sandbox event observers
type Observer interface {
	OnEvent(event SandboxEvent)
}

// EventManager manages sandbox events and observers
type EventManager struct {
	observers []Observer
}

// NewEventManager creates a new event manager
func NewEventManager() *EventManager {
	return &EventManager{
		observers: make([]Observer, 0),
	}
}

// AddObserver adds an event observer
func (em *EventManager) AddObserver(observer Observer) {
	em.observers = append(em.observers, observer)
}

// RemoveObserver removes an event observer
func (em *EventManager) RemoveObserver(observer Observer) {
	for i, obs := range em.observers {
		if obs == observer {
			em.observers = append(em.observers[:i], em.observers[i+1:]...)
			break
		}
	}
}

// Emit emits an event to all observers
func (em *EventManager) Emit(event SandboxEvent) {
	for _, observer := range em.observers {
		observer.OnEvent(event)
	}
}
