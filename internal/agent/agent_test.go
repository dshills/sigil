package agent

import (
	"context"
	"testing"
	"time"

	"github.com/dshills/sigil/internal/model"
	"github.com/dshills/sigil/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockModel implements model.Model for testing
type MockModel struct {
	mock.Mock
}

func (m *MockModel) RunPrompt(ctx context.Context, input model.PromptInput) (model.PromptOutput, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(model.PromptOutput), args.Error(1)
}

func (m *MockModel) GetCapabilities() model.ModelCapabilities {
	args := m.Called()
	return args.Get(0).(model.ModelCapabilities)
}

func (m *MockModel) Name() string {
	args := m.Called()
	return args.String(0)
}

// MockSandboxManager implements sandbox.Manager for testing
type MockSandboxManager struct {
	mock.Mock
}

func (m *MockSandboxManager) ExecuteCode(ctx context.Context, request sandbox.ExecutionRequest) (*sandbox.ExecutionResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sandbox.ExecutionResponse), args.Error(1)
}

func (m *MockSandboxManager) ValidateCode(path string, content string) error {
	args := m.Called(path, content)
	return args.Error(0)
}

func (m *MockSandboxManager) GetValidationRules(path string) ([]sandbox.FileRule, []sandbox.ContentRule) {
	args := m.Called(path)
	return args.Get(0).([]sandbox.FileRule), args.Get(1).([]sandbox.ContentRule)
}

func (m *MockSandboxManager) CreateSandbox() (sandbox.Sandbox, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(sandbox.Sandbox), args.Error(1)
}

func (m *MockSandboxManager) ListSandboxes() []sandbox.SandboxInfo {
	args := m.Called()
	return args.Get(0).([]sandbox.SandboxInfo)
}

func (m *MockSandboxManager) Cleanup() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewBaseAgent(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	capabilities := []Capability{CapabilityCodeGeneration, CapabilityCodeReview}
	config := AgentConfig{
		Role:         RoleLead,
		Model:        "test-model",
		Capabilities: capabilities,
		Priority:     1,
		Enabled:      true,
	}

	agent := NewBaseAgent("test-id", RoleLead, mockModel, capabilities, config, mockSandbox)

	assert.NotNil(t, agent)
	assert.Equal(t, "test-id", agent.GetID())
	assert.Equal(t, RoleLead, agent.GetRole())
	assert.Equal(t, capabilities, agent.GetCapabilities())
	assert.Equal(t, mockModel, agent.GetModel())
}

func TestBaseAgent_HasCapability(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	capabilities := []Capability{CapabilityCodeGeneration, CapabilityCodeReview}
	config := AgentConfig{}

	agent := NewBaseAgent("test-id", RoleLead, mockModel, capabilities, config, mockSandbox)

	t.Run("has capability", func(t *testing.T) {
		assert.True(t, agent.HasCapability(CapabilityCodeGeneration))
		assert.True(t, agent.HasCapability(CapabilityCodeReview))
	})

	t.Run("does not have capability", func(t *testing.T) {
		assert.False(t, agent.HasCapability(CapabilityTesting))
		assert.False(t, agent.HasCapability(CapabilityDocumentation))
	})
}

func TestNewLeadAgent(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleLead,
		Model:   "test-model",
		Enabled: true,
	}

	agent := NewLeadAgent("lead-1", mockModel, config, mockSandbox)

	assert.NotNil(t, agent)
	assert.Equal(t, "lead-1", agent.GetID())
	assert.Equal(t, RoleLead, agent.GetRole())
	assert.Equal(t, mockModel, agent.GetModel())
	
	// Check that lead agent has expected capabilities
	expectedCapabilities := []Capability{
		CapabilityCodeGeneration,
		CapabilityRefactoring,
		CapabilityDocumentation,
	}
	
	capabilities := agent.GetCapabilities()
	assert.Len(t, capabilities, len(expectedCapabilities))
	for _, cap := range expectedCapabilities {
		assert.True(t, agent.HasCapability(cap))
	}
}

func TestTaskTypes(t *testing.T) {
	tests := []struct {
		name     string
		taskType TaskType
		expected string
	}{
		{"edit task", TaskTypeEdit, "edit"},
		{"generate task", TaskTypeGenerate, "generate"},
		{"refactor task", TaskTypeRefactor, "refactor"},
		{"document task", TaskTypeDocument, "document"},
		{"test task", TaskTypeTest, "test"},
		{"review task", TaskTypeReview, "review"},
		{"optimize task", TaskTypeOptimize, "optimize"},
		{"analyze task", TaskTypeAnalyze, "analyze"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.taskType))
		})
	}
}

func TestAgentRoles(t *testing.T) {
	tests := []struct {
		name     string
		role     AgentRole
		expected string
	}{
		{"lead role", RoleLead, "lead"},
		{"reviewer role", RoleReviewer, "reviewer"},
		{"expert role", RoleExpert, "expert"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.role))
		})
	}
}

func TestCapabilities(t *testing.T) {
	tests := []struct {
		name       string
		capability Capability
		expected   string
	}{
		{"code generation", CapabilityCodeGeneration, "code_generation"},
		{"code review", CapabilityCodeReview, "code_review"},
		{"testing", CapabilityTesting, "testing"},
		{"documentation", CapabilityDocumentation, "documentation"},
		{"refactoring", CapabilityRefactoring, "refactoring"},
		{"security analysis", CapabilitySecurityAnalysis, "security_analysis"},
		{"performance analysis", CapabilityPerformanceAnalysis, "performance_analysis"},
		{"architecture review", CapabilityArchitectureReview, "architecture_review"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.capability))
		})
	}
}

func TestPriorities(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
		expected string
	}{
		{"low priority", PriorityLow, "low"},
		{"medium priority", PriorityMedium, "medium"},
		{"high priority", PriorityHigh, "high"},
		{"critical priority", PriorityCritical, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.priority))
		})
	}
}

func TestResultStatuses(t *testing.T) {
	tests := []struct {
		name     string
		status   ResultStatus
		expected string
	}{
		{"success status", StatusSuccess, "success"},
		{"partial status", StatusPartial, "partial"},
		{"failed status", StatusFailed, "failed"},
		{"incomplete status", StatusIncomplete, "incomplete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestReviewDecisions(t *testing.T) {
	tests := []struct {
		name     string
		decision ReviewDecision
		expected string
	}{
		{"approve decision", DecisionApprove, "approve"},
		{"request changes decision", DecisionRequestChanges, "request_changes"},
		{"reject decision", DecisionReject, "reject"},
		{"needs more info decision", DecisionNeedsMoreInfo, "needs_more_info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.decision))
		})
	}
}

func TestTask_Creation(t *testing.T) {
	task := Task{
		ID:          "task-1",
		Type:        TaskTypeEdit,
		Description: "Edit main function",
		Priority:    PriorityHigh,
		CreatedAt:   time.Now(),
		Context: TaskContext{
			Files: []FileContext{
				{
					Path:     "main.go",
					Content:  "package main",
					Language: "go",
					IsTarget: true,
				},
			},
			Requirements: []string{"Add error handling"},
		},
	}

	assert.Equal(t, "task-1", task.ID)
	assert.Equal(t, TaskTypeEdit, task.Type)
	assert.Equal(t, "Edit main function", task.Description)
	assert.Equal(t, PriorityHigh, task.Priority)
	assert.Len(t, task.Context.Files, 1)
	assert.Equal(t, "main.go", task.Context.Files[0].Path)
	assert.True(t, task.Context.Files[0].IsTarget)
	assert.Len(t, task.Context.Requirements, 1)
}

func TestProposal_Creation(t *testing.T) {
	proposal := Proposal{
		ID:          "proposal-1",
		AgentID:     "agent-1",
		Type:        ProposalTypeFileChange,
		Description: "Add error handling to main function",
		Confidence:  0.9,
		CreatedAt:   time.Now(),
		Changes: []Change{
			{
				Type:        ChangeTypeUpdate,
				Path:        "main.go",
				OldContent:  "func main() {",
				NewContent:  "func main() error {",
				StartLine:   5,
				Description: "Add error return to main function",
			},
		},
		Impact: Impact{
			Scope: ScopeLocal,
			Risk:  RiskLow,
			Benefits: []string{"Better error handling"},
		},
	}

	assert.Equal(t, "proposal-1", proposal.ID)
	assert.Equal(t, "agent-1", proposal.AgentID)
	assert.Equal(t, ProposalTypeFileChange, proposal.Type)
	assert.Equal(t, 0.9, proposal.Confidence)
	assert.Len(t, proposal.Changes, 1)
	assert.Equal(t, ChangeTypeUpdate, proposal.Changes[0].Type)
	assert.Equal(t, ScopeLocal, proposal.Impact.Scope)
	assert.Equal(t, RiskLow, proposal.Impact.Risk)
}

func TestResult_Creation(t *testing.T) {
	result := Result{
		TaskID:     "task-1",
		AgentID:    "agent-1",
		Status:     StatusSuccess,
		Confidence: 0.85,
		Duration:   2 * time.Second,
		Timestamp:  time.Now(),
		Reasoning:  "Successfully completed task",
		Proposals: []Proposal{
			{
				ID:      "proposal-1",
				AgentID: "agent-1",
				Type:    ProposalTypeFileChange,
			},
		},
	}

	assert.Equal(t, "task-1", result.TaskID)
	assert.Equal(t, "agent-1", result.AgentID)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, 0.85, result.Confidence)
	assert.Equal(t, 2*time.Second, result.Duration)
	assert.Len(t, result.Proposals, 1)
}

func TestReviewResult_Creation(t *testing.T) {
	reviewResult := ReviewResult{
		ProposalID: "proposal-1",
		ReviewerID: "reviewer-1",
		Decision:   DecisionApprove,
		Score:      0.8,
		Confidence: 0.9,
		Reasoning:  "Good implementation",
		Timestamp:  time.Now(),
		Comments: []ReviewComment{
			{
				Type:     CommentTypeGeneral,
				Severity: SeverityInfo,
				Message:  "Looks good overall",
			},
		},
	}

	assert.Equal(t, "proposal-1", reviewResult.ProposalID)
	assert.Equal(t, "reviewer-1", reviewResult.ReviewerID)
	assert.Equal(t, DecisionApprove, reviewResult.Decision)
	assert.Equal(t, 0.8, reviewResult.Score)
	assert.Equal(t, 0.9, reviewResult.Confidence)
	assert.Len(t, reviewResult.Comments, 1)
	assert.Equal(t, CommentTypeGeneral, reviewResult.Comments[0].Type)
}

func TestDefaultOrchestrationConfig(t *testing.T) {
	config := DefaultOrchestrationConfig()

	assert.Equal(t, 5, config.MaxAgents)
	assert.Equal(t, 0.7, config.ConsensusThreshold)
	assert.Equal(t, ResolutionVoting, config.ConflictResolution)
	assert.Equal(t, 10*time.Minute, config.TaskTimeout)
	assert.Equal(t, 5*time.Minute, config.ReviewTimeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.True(t, config.EnableParallelReview)
	
	// Check quality gate
	assert.Equal(t, 0.8, config.QualityGate.MinConfidence)
	assert.Equal(t, 2, config.QualityGate.MinReviewers)
	assert.Equal(t, 4, config.QualityGate.MaxReviewers)
	assert.Contains(t, config.QualityGate.RequiredCapabilities, CapabilityCodeReview)
	
	// Check agent profiles
	assert.Contains(t, config.AgentProfiles, "lead")
	assert.Contains(t, config.AgentProfiles, "reviewer")
	
	leadProfile := config.AgentProfiles["lead"]
	assert.Equal(t, RoleLead, leadProfile.Role)
	assert.Equal(t, "claude-3-5-sonnet-20241022", leadProfile.Model)
	assert.Contains(t, leadProfile.Capabilities, CapabilityCodeGeneration)
	assert.True(t, leadProfile.Enabled)
	
	reviewerProfile := config.AgentProfiles["reviewer"]
	assert.Equal(t, RoleReviewer, reviewerProfile.Role)
	assert.Equal(t, "gpt-4", reviewerProfile.Model)
	assert.Contains(t, reviewerProfile.Capabilities, CapabilityCodeReview)
	assert.True(t, reviewerProfile.Enabled)
}

func TestConstraintTypes(t *testing.T) {
	tests := []struct {
		name           string
		constraintType ConstraintType
		expected       string
	}{
		{"style constraint", ConstraintTypeStyle, "style"},
		{"security constraint", ConstraintTypeSecurity, "security"},
		{"performance constraint", ConstraintTypePerformance, "performance"},
		{"compatibility constraint", ConstraintTypeCompatibility, "compatibility"},
		{"resource constraint", ConstraintTypeResource, "resource"},
		{"testing constraint", ConstraintTypeTesting, "testing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.constraintType))
		})
	}
}

func TestSeverityLevels(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		expected string
	}{
		{"info severity", SeverityInfo, "info"},
		{"warning severity", SeverityWarning, "warning"},
		{"error severity", SeverityError, "error"},
		{"critical severity", SeverityCritical, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.severity))
		})
	}
}

func TestSpecializationConstants(t *testing.T) {
	assert.Equal(t, "security", SpecializationSecurity)
	assert.Equal(t, "performance", SpecializationPerformance)
	assert.Equal(t, "architecture", SpecializationArchitecture)
	assert.Equal(t, "testing", SpecializationTesting)
}