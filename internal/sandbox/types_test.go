package sandbox

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExecutorConfig_Structure(t *testing.T) {
	config := ExecutorConfig{
		Timeout:         5 * time.Minute,
		MaxWorktrees:    10,
		CleanupInterval: 1 * time.Hour,
		AllowedCommands: []string{"go", "npm", "python"},
		BlockedCommands: []string{"rm", "sudo"},
		WorkingDir:      ".sigil/sandbox",
	}

	assert.Equal(t, 5*time.Minute, config.Timeout)
	assert.Equal(t, 10, config.MaxWorktrees)
	assert.Equal(t, 1*time.Hour, config.CleanupInterval)
	assert.Len(t, config.AllowedCommands, 3)
	assert.Contains(t, config.AllowedCommands, "go")
	assert.Len(t, config.BlockedCommands, 2)
	assert.Contains(t, config.BlockedCommands, "rm")
	assert.Equal(t, ".sigil/sandbox", config.WorkingDir)
}

func TestDefaultExecutorConfig(t *testing.T) {
	config := DefaultExecutorConfig()

	assert.Equal(t, 5*time.Minute, config.Timeout)
	assert.Equal(t, 10, config.MaxWorktrees)
	assert.Equal(t, 1*time.Hour, config.CleanupInterval)
	assert.NotEmpty(t, config.AllowedCommands)
	assert.NotEmpty(t, config.BlockedCommands)
	assert.Equal(t, ".sigil/sandbox", config.WorkingDir)

	// Check some expected allowed commands
	assert.Contains(t, config.AllowedCommands, "go")
	assert.Contains(t, config.AllowedCommands, "npm")
	assert.Contains(t, config.AllowedCommands, "python")
	assert.Contains(t, config.AllowedCommands, "git")

	// Check some expected blocked commands
	assert.Contains(t, config.BlockedCommands, "rm")
	assert.Contains(t, config.BlockedCommands, "sudo")
	assert.Contains(t, config.BlockedCommands, "curl")
}

func TestSandboxInfo_Structure(t *testing.T) {
	createdAt := time.Now().Add(-1 * time.Hour)
	lastUsed := time.Now()

	info := SandboxInfo{
		ID:        "sandbox-123",
		Path:      "/tmp/sandbox-123",
		CreatedAt: createdAt,
		LastUsed:  lastUsed,
		Status:    SandboxStatusActive,
	}

	assert.Equal(t, "sandbox-123", info.ID)
	assert.Equal(t, "/tmp/sandbox-123", info.Path)
	assert.Equal(t, createdAt, info.CreatedAt)
	assert.Equal(t, lastUsed, info.LastUsed)
	assert.Equal(t, SandboxStatusActive, info.Status)
}

func TestSandboxStatus_Constants(t *testing.T) {
	assert.Equal(t, SandboxStatus("active"), SandboxStatusActive)
	assert.Equal(t, SandboxStatus("idle"), SandboxStatusIdle)
	assert.Equal(t, SandboxStatus("cleaned"), SandboxStatusCleaned)
}

func TestValidationConfig_Structure(t *testing.T) {
	config := ValidationConfig{
		Enabled:      true,
		RulesFile:    ".sigil/rules.yml",
		StrictMode:   false,
		Timeout:      30 * time.Second,
		MaxFileSize:  1024 * 1024,
		MaxTotalSize: 10 * 1024 * 1024,
		MaxFiles:     100,
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, ".sigil/rules.yml", config.RulesFile)
	assert.False(t, config.StrictMode)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, int64(1024*1024), config.MaxFileSize)
	assert.Equal(t, int64(10*1024*1024), config.MaxTotalSize)
	assert.Equal(t, 100, config.MaxFiles)
}

func TestDefaultValidationConfig(t *testing.T) {
	config := DefaultValidationConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, ".sigil/rules.yml", config.RulesFile)
	assert.False(t, config.StrictMode)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, int64(1024*1024), config.MaxFileSize)
	assert.Equal(t, int64(10*1024*1024), config.MaxTotalSize)
	assert.Equal(t, 100, config.MaxFiles)
}

func TestTestConfiguration_Structure(t *testing.T) {
	config := TestConfiguration{
		Command:     "go",
		Args:        []string{"test", "./..."},
		Directory:   ".",
		Environment: map[string]string{"GO_ENV": "test"},
		Timeout:     5 * time.Minute,
	}

	assert.Equal(t, "go", config.Command)
	assert.Len(t, config.Args, 2)
	assert.Equal(t, "test", config.Args[0])
	assert.Equal(t, "./...", config.Args[1])
	assert.Equal(t, ".", config.Directory)
	assert.Equal(t, "test", config.Environment["GO_ENV"])
	assert.Equal(t, 5*time.Minute, config.Timeout)
}

func TestBuildConfiguration_Structure(t *testing.T) {
	config := BuildConfiguration{
		Command:     "go",
		Args:        []string{"build", "./..."},
		Directory:   ".",
		Environment: map[string]string{"CGO_ENABLED": "0"},
		Timeout:     5 * time.Minute,
		Artifacts:   []string{"bin/app", "dist/"},
	}

	assert.Equal(t, "go", config.Command)
	assert.Len(t, config.Args, 2)
	assert.Equal(t, ".", config.Directory)
	assert.Equal(t, "0", config.Environment["CGO_ENABLED"])
	assert.Equal(t, 5*time.Minute, config.Timeout)
	assert.Len(t, config.Artifacts, 2)
	assert.Contains(t, config.Artifacts, "bin/app")
}

func TestLintConfiguration_Structure(t *testing.T) {
	config := LintConfiguration{
		Command:     "golangci-lint",
		Args:        []string{"run"},
		Directory:   ".",
		Environment: map[string]string{"GOLANGCI_LINT_CACHE": "/tmp"},
		Timeout:     5 * time.Minute,
		ConfigFile:  ".golangci.yml",
	}

	assert.Equal(t, "golangci-lint", config.Command)
	assert.Len(t, config.Args, 1)
	assert.Equal(t, "run", config.Args[0])
	assert.Equal(t, ".", config.Directory)
	assert.Equal(t, "/tmp", config.Environment["GOLANGCI_LINT_CACHE"])
	assert.Equal(t, 5*time.Minute, config.Timeout)
	assert.Equal(t, ".golangci.yml", config.ConfigFile)
}

func TestProjectConfiguration_Structure(t *testing.T) {
	config := ProjectConfiguration{
		Name:      "Test Project",
		Language:  "go",
		Framework: "",
		Test: TestConfiguration{
			Command: "go",
			Args:    []string{"test", "./..."},
		},
		Build: BuildConfiguration{
			Command: "go",
			Args:    []string{"build", "./..."},
		},
		Lint: LintConfiguration{
			Command: "golangci-lint",
			Args:    []string{"run"},
		},
		Validation:  DefaultValidationConfig(),
		Environment: map[string]string{"ENV": "test"},
	}

	assert.Equal(t, "Test Project", config.Name)
	assert.Equal(t, "go", config.Language)
	assert.Empty(t, config.Framework)
	assert.Equal(t, "go", config.Test.Command)
	assert.Equal(t, "go", config.Build.Command)
	assert.Equal(t, "golangci-lint", config.Lint.Command)
	assert.True(t, config.Validation.Enabled)
	assert.Equal(t, "test", config.Environment["ENV"])
}

func TestDefaultProjectConfigurations(t *testing.T) {
	configs := DefaultProjectConfigurations()

	// Check that expected configurations exist
	assert.Contains(t, configs, "go")
	assert.Contains(t, configs, "node")
	assert.Contains(t, configs, "python")

	// Check Go configuration
	goConfig := configs["go"]
	assert.Equal(t, "Go Project", goConfig.Name)
	assert.Equal(t, "go", goConfig.Language)
	assert.Equal(t, "go", goConfig.Test.Command)
	assert.Equal(t, "go", goConfig.Build.Command)
	assert.Equal(t, "golangci-lint", goConfig.Lint.Command)

	// Check Node configuration
	nodeConfig := configs["node"]
	assert.Equal(t, "Node.js Project", nodeConfig.Name)
	assert.Equal(t, "javascript", nodeConfig.Language)
	assert.Equal(t, "node", nodeConfig.Framework)
	assert.Equal(t, "npm", nodeConfig.Test.Command)

	// Check Python configuration
	pythonConfig := configs["python"]
	assert.Equal(t, "Python Project", pythonConfig.Name)
	assert.Equal(t, "python", pythonConfig.Language)
	assert.Equal(t, "python", pythonConfig.Test.Command)
	assert.Contains(t, pythonConfig.Test.Args, "-m")
	assert.Contains(t, pythonConfig.Test.Args, "pytest")
}

func TestSandboxMetrics_Structure(t *testing.T) {
	lastCleanup := time.Now().Add(-1 * time.Hour)

	metrics := SandboxMetrics{
		TotalSandboxes:  10,
		ActiveSandboxes: 5,
		TotalExecutions: 100,
		SuccessfulRuns:  95,
		FailedRuns:      5,
		AverageExecTime: 2 * time.Second,
		TotalCleanups:   8,
		DiskUsage:       1024 * 1024,
		LastCleanupTime: lastCleanup,
	}

	assert.Equal(t, 10, metrics.TotalSandboxes)
	assert.Equal(t, 5, metrics.ActiveSandboxes)
	assert.Equal(t, int64(100), metrics.TotalExecutions)
	assert.Equal(t, int64(95), metrics.SuccessfulRuns)
	assert.Equal(t, int64(5), metrics.FailedRuns)
	assert.Equal(t, 2*time.Second, metrics.AverageExecTime)
	assert.Equal(t, int64(8), metrics.TotalCleanups)
	assert.Equal(t, int64(1024*1024), metrics.DiskUsage)
	assert.Equal(t, lastCleanup, metrics.LastCleanupTime)
}

func TestSandboxEvent_Structure(t *testing.T) {
	timestamp := time.Now()

	event := SandboxEvent{
		Type:      EventSandboxCreated,
		SandboxID: "sandbox-123",
		Timestamp: timestamp,
		Data:      map[string]string{"reason": "test"},
		Error:     "",
	}

	assert.Equal(t, EventSandboxCreated, event.Type)
	assert.Equal(t, "sandbox-123", event.SandboxID)
	assert.Equal(t, timestamp, event.Timestamp)
	assert.Equal(t, "test", event.Data["reason"])
	assert.Empty(t, event.Error)
}

func TestEventType_Constants(t *testing.T) {
	assert.Equal(t, EventType("sandbox_created"), EventSandboxCreated)
	assert.Equal(t, EventType("sandbox_cleaned"), EventSandboxCleaned)
	assert.Equal(t, EventType("execution_started"), EventExecutionStarted)
	assert.Equal(t, EventType("execution_ended"), EventExecutionEnded)
	assert.Equal(t, EventType("validation_failed"), EventValidationFailed)
	assert.Equal(t, EventType("timeout_reached"), EventTimeoutReached)
}

func TestEventManager_Basic(t *testing.T) {
	manager := NewEventManager()
	assert.NotNil(t, manager)
	assert.Empty(t, manager.observers)
}

func TestEventManager_AddRemoveObserver(t *testing.T) {
	manager := NewEventManager()
	observer := &MockObserver{}

	// Add observer
	manager.AddObserver(observer)
	assert.Len(t, manager.observers, 1)
	assert.Equal(t, observer, manager.observers[0])

	// Remove observer
	manager.RemoveObserver(observer)
	assert.Empty(t, manager.observers)
}

func TestEventManager_Emit(t *testing.T) {
	manager := NewEventManager()
	observer1 := &MockObserver{}
	observer2 := &MockObserver{}

	manager.AddObserver(observer1)
	manager.AddObserver(observer2)

	event := SandboxEvent{
		Type:      EventSandboxCreated,
		SandboxID: "test-123",
		Timestamp: time.Now(),
	}

	manager.Emit(event)

	// Check that both observers received the event
	assert.Len(t, observer1.Events, 1)
	assert.Len(t, observer2.Events, 1)
	assert.Equal(t, event, observer1.Events[0])
	assert.Equal(t, event, observer2.Events[0])
}

// MockObserver implements Observer for testing
type MockObserver struct {
	Events []SandboxEvent
}

func (m *MockObserver) OnEvent(event SandboxEvent) {
	m.Events = append(m.Events, event)
}

func TestExecutionStatus_Constants(t *testing.T) {
	assert.Equal(t, ExecutionStatus("pending"), StatusPending)
	assert.Equal(t, ExecutionStatus("running"), StatusRunning)
	assert.Equal(t, ExecutionStatus("completed"), StatusCompleted)
	assert.Equal(t, ExecutionStatus("failed"), StatusFailed)
	assert.Equal(t, ExecutionStatus("timeout"), StatusTimeout)
}

func TestFileOperation_Constants(t *testing.T) {
	assert.Equal(t, FileOperation("create"), OperationCreate)
	assert.Equal(t, FileOperation("update"), OperationUpdate)
	assert.Equal(t, FileOperation("delete"), OperationDelete)
}

func TestExecutionRequest_Structure(t *testing.T) {
	request := ExecutionRequest{
		ID:   "req-123",
		Type: "validation",
		Files: []FileChange{
			{
				Path:      "main.go",
				Content:   "package main",
				Operation: OperationCreate,
			},
		},
		ValidationSteps: []ValidationStep{
			{
				Name:        "Build",
				Command:     "go",
				Args:        []string{"build", "."},
				Required:    true,
				Description: "Build the application",
			},
		},
		Context: map[string]string{"branch": "main"},
	}

	assert.Equal(t, "req-123", request.ID)
	assert.Equal(t, "validation", request.Type)
	assert.Len(t, request.Files, 1)
	assert.Equal(t, "main.go", request.Files[0].Path)
	assert.Equal(t, OperationCreate, request.Files[0].Operation)
	assert.Len(t, request.ValidationSteps, 1)
	assert.Equal(t, "Build", request.ValidationSteps[0].Name)
	assert.True(t, request.ValidationSteps[0].Required)
	assert.Equal(t, "main", request.Context["branch"])
}

func TestFileChange_Structure(t *testing.T) {
	change := FileChange{
		Path:      "test.go",
		Content:   "package test",
		Operation: OperationUpdate,
	}

	assert.Equal(t, "test.go", change.Path)
	assert.Equal(t, "package test", change.Content)
	assert.Equal(t, OperationUpdate, change.Operation)
}

func TestValidationStep_Structure(t *testing.T) {
	step := ValidationStep{
		Name:        "Test",
		Command:     "go",
		Args:        []string{"test", "./..."},
		Required:    true,
		Description: "Run tests",
	}

	assert.Equal(t, "Test", step.Name)
	assert.Equal(t, "go", step.Command)
	assert.Len(t, step.Args, 2)
	assert.Equal(t, "test", step.Args[0])
	assert.True(t, step.Required)
	assert.Equal(t, "Run tests", step.Description)
}

func TestExecutionResponse_Structure(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(5 * time.Second)

	response := ExecutionResponse{
		RequestID:  "req-123",
		WorktreeID: "wt-456",
		Status:     StatusCompleted,
		StartTime:  startTime,
		EndTime:    endTime,
		Results: []ExecutionResult{
			{
				Command:  "go test",
				Output:   "ok",
				ExitCode: 0,
			},
		},
		Diff:  "diff content",
		Error: "",
	}

	assert.Equal(t, "req-123", response.RequestID)
	assert.Equal(t, "wt-456", response.WorktreeID)
	assert.Equal(t, StatusCompleted, response.Status)
	assert.Equal(t, startTime, response.StartTime)
	assert.Equal(t, endTime, response.EndTime)
	assert.Len(t, response.Results, 1)
	assert.Equal(t, "go test", response.Results[0].Command)
	assert.Equal(t, "diff content", response.Diff)
	assert.Empty(t, response.Error)
}

func TestExecutionResponse_Duration(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(5 * time.Second)

	response := ExecutionResponse{
		StartTime: startTime,
		EndTime:   endTime,
	}

	duration := response.Duration()
	assert.Equal(t, 5*time.Second, duration)

	// Test with zero end time (running)
	response.EndTime = time.Time{}
	duration = response.Duration()
	assert.True(t, duration > 0) // Should be time since start
}

func TestExecutionResponse_Success(t *testing.T) {
	response := ExecutionResponse{Status: StatusCompleted}
	assert.True(t, response.Success())

	response.Status = StatusFailed
	assert.False(t, response.Success())

	response.Status = StatusRunning
	assert.False(t, response.Success())
}

func TestProjectConfiguration_LanguageSpecificDefaults(t *testing.T) {
	configs := DefaultProjectConfigurations()

	// Test Go configuration specifics
	goConfig := configs["go"]
	assert.Equal(t, 5*time.Minute, goConfig.Test.Timeout)
	assert.Equal(t, 5*time.Minute, goConfig.Build.Timeout)
	assert.Equal(t, ".golangci.yml", goConfig.Lint.ConfigFile)

	// Test Node configuration specifics
	nodeConfig := configs["node"]
	assert.Contains(t, nodeConfig.Test.Args, "test")
	assert.Contains(t, nodeConfig.Build.Args, "build")
	assert.Contains(t, nodeConfig.Lint.Args, "lint")

	// Test Python configuration specifics
	pythonConfig := configs["python"]
	assert.Contains(t, pythonConfig.Test.Args, "pytest")
	assert.Contains(t, pythonConfig.Lint.Args, "flake8")
	assert.Equal(t, 10*time.Minute, pythonConfig.Test.Timeout) // Python tests can be slower
}

func TestValidationConfig_Limits(t *testing.T) {
	config := DefaultValidationConfig()

	// Test size limits are reasonable
	assert.Equal(t, int64(1024*1024), config.MaxFileSize)      // 1MB
	assert.Equal(t, int64(10*1024*1024), config.MaxTotalSize) // 10MB
	assert.Equal(t, 100, config.MaxFiles)

	// Test timeout is reasonable
	assert.Equal(t, 30*time.Second, config.Timeout)

	// Test defaults
	assert.True(t, config.Enabled)
	assert.False(t, config.StrictMode)
}