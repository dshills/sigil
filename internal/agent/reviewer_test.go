package agent

import (
	"context"
	"testing"
	"time"

	"github.com/dshills/sigil/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewReviewerAgent(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:         RoleReviewer,
		Model:        "test-model",
		Capabilities: []Capability{CapabilityCodeReview},
		Priority:     2,
		Enabled:      true,
	}
	
	reviewer := NewReviewerAgent("reviewer-1", mockModel, config, mockSandbox, SpecializationSecurity)
	
	assert.NotNil(t, reviewer)
	assert.Equal(t, "reviewer-1", reviewer.GetID())
	assert.Equal(t, RoleReviewer, reviewer.GetRole())
	assert.Equal(t, SpecializationSecurity, reviewer.specialization)
	assert.Equal(t, mockModel, reviewer.GetModel())
	
	// Check base capabilities
	assert.True(t, reviewer.HasCapability(CapabilityCodeReview))
	assert.True(t, reviewer.HasCapability(CapabilityTesting))
	assert.True(t, reviewer.HasCapability(CapabilitySecurityAnalysis))
}

func TestNewReviewerAgent_SecuritySpecialization(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	reviewer := NewReviewerAgent("security-reviewer", mockModel, config, mockSandbox, SpecializationSecurity)
	
	assert.NotNil(t, reviewer)
	assert.Equal(t, SpecializationSecurity, reviewer.specialization)
	assert.True(t, reviewer.HasCapability(CapabilitySecurityAnalysis))
	assert.True(t, reviewer.HasCapability(CapabilityCodeReview))
}

func TestNewReviewerAgent_PerformanceSpecialization(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	reviewer := NewReviewerAgent("perf-reviewer", mockModel, config, mockSandbox, SpecializationPerformance)
	
	assert.NotNil(t, reviewer)
	assert.Equal(t, SpecializationPerformance, reviewer.specialization)
	assert.True(t, reviewer.HasCapability(CapabilityPerformanceAnalysis))
	assert.True(t, reviewer.HasCapability(CapabilityCodeReview))
}

func TestNewReviewerAgent_ArchitectureSpecialization(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	reviewer := NewReviewerAgent("arch-reviewer", mockModel, config, mockSandbox, SpecializationArchitecture)
	
	assert.NotNil(t, reviewer)
	assert.Equal(t, SpecializationArchitecture, reviewer.specialization)
	assert.True(t, reviewer.HasCapability(CapabilityArchitectureReview))
	assert.True(t, reviewer.HasCapability(CapabilityCodeReview))
}

func TestNewReviewerAgent_UnknownSpecialization(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	reviewer := NewReviewerAgent("general-reviewer", mockModel, config, mockSandbox, "unknown")
	
	assert.NotNil(t, reviewer)
	assert.Equal(t, "unknown", reviewer.specialization)
	// Should still have base capabilities
	assert.True(t, reviewer.HasCapability(CapabilityCodeReview))
	assert.True(t, reviewer.HasCapability(CapabilityTesting))
	assert.True(t, reviewer.HasCapability(CapabilitySecurityAnalysis))
}

func TestReviewerAgent_Execute_ReviewTask(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	reviewer := NewReviewerAgent("reviewer-1", mockModel, config, mockSandbox, SpecializationSecurity)
	
	// Create a review task
	task := Task{
		ID:          "review-task-1",
		Type:        TaskTypeReview,
		Description: "Review security aspects of authentication code",
		Priority:    PriorityHigh,
		CreatedAt:   time.Now(),
		Context: TaskContext{
			Files: []FileContext{
				{
					Path:     "auth.go",
					Content:  "package auth\n\nfunc Login(user, pass string) bool {\n\treturn user == \"admin\" && pass == \"password\"\n}",
					Language: "go",
					IsTarget: true,
				},
			},
			Requirements: []string{"Check for security vulnerabilities"},
		},
	}
	
	// Mock the model response
	expectedResponse := model.PromptOutput{
		Response: "Security Review:\n1. Hardcoded credentials detected\n2. No password hashing\n3. Recommend using bcrypt",
		Metadata: map[string]string{
			"confidence": "0.9",
		},
	}
	
	mockModel.On("RunPrompt", mock.Anything, mock.Anything).Return(expectedResponse, nil)
	
	ctx := context.Background()
	result, err := reviewer.Execute(ctx, task)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "review-task-1", result.TaskID)
	assert.Equal(t, "reviewer-1", result.AgentID)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.True(t, result.Confidence > 0.8) // Should have high confidence
	assert.Contains(t, result.Reasoning, "Completed security review analysis")
	
	mockModel.AssertExpectations(t)
}

func TestReviewerAgent_Execute_NonReviewTask(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	reviewer := NewReviewerAgent("reviewer-1", mockModel, config, mockSandbox, SpecializationSecurity)
	
	// Create a non-review task (code generation)
	task := Task{
		ID:          "generate-task-1",
		Type:        TaskTypeGenerate,
		Description: "Generate new authentication function",
		Priority:    PriorityMedium,
		CreatedAt:   time.Now(),
	}
	
	ctx := context.Background()
	result, err := reviewer.Execute(ctx, task)
	
	// Reviewer should handle non-review tasks but with lower confidence
	// or delegate to base agent functionality
	if err != nil {
		// If it returns an error, it should be about unsupported task type
		assert.Contains(t, err.Error(), "task type")
	} else {
		// If it handles the task, confidence should be lower
		assert.NotNil(t, result)
		assert.Equal(t, "generate-task-1", result.TaskID)
		assert.Equal(t, "reviewer-1", result.AgentID)
	}
}

func TestReviewerAgent_Execute_ModelError(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	reviewer := NewReviewerAgent("reviewer-1", mockModel, config, mockSandbox, SpecializationSecurity)
	
	task := Task{
		ID:          "review-task-1",
		Type:        TaskTypeReview,
		Description: "Review code",
		Priority:    PriorityHigh,
		CreatedAt:   time.Now(),
		Context: TaskContext{
			Files: []FileContext{
				{
					Path:     "test.go",
					Content:  "package test",
					Language: "go",
					IsTarget: true,
				},
			},
		},
	}
	
	// Mock model to return an error
	mockModel.On("RunPrompt", mock.Anything, mock.Anything).Return(model.PromptOutput{}, assert.AnError)
	
	ctx := context.Background()
	result, err := reviewer.Execute(ctx, task)
	
	assert.Error(t, err)
	assert.NotNil(t, result) // Implementation returns failed result rather than nil
	assert.Equal(t, StatusFailed, result.Status)
	
	mockModel.AssertExpectations(t)
}

func TestReviewerAgent_Review(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	reviewer := NewReviewerAgent("reviewer-1", mockModel, config, mockSandbox, SpecializationSecurity)
	
	// Test the ReviewCode method specifically
	proposal := Proposal{
		ID:          "proposal-1",
		AgentID:     "lead-1",
		Type:        ProposalTypeFileChange,
		Description: "Add authentication check",
		Confidence:  0.8,
		CreatedAt:   time.Now(),
		Changes: []Change{
			{
				Type:        ChangeTypeUpdate,
				Path:        "main.go",
				OldContent:  "func handler() {}",
				NewContent:  "func handler() {\n\tif !isAuthenticated() {\n\t\treturn\n\t}\n}",
				StartLine:   10,
				Description: "Add authentication check",
			},
		},
	}
	
	// Mock model response for review
	reviewResponse := model.PromptOutput{
		Response: "APPROVE: Good security practice. Authentication check is properly implemented.",
		Metadata: map[string]string{
			"confidence": "0.85",
			"decision":   "approve",
		},
	}
	
	mockModel.On("RunPrompt", mock.Anything, mock.Anything).Return(reviewResponse, nil)
	
	ctx := context.Background()
	reviewResult, err := reviewer.Review(ctx, proposal)
	
	assert.NoError(t, err)
	assert.NotNil(t, reviewResult)
	assert.Equal(t, "proposal-1", reviewResult.ProposalID)
	assert.Equal(t, "reviewer-1", reviewResult.ReviewerID)
	assert.Equal(t, DecisionApprove, reviewResult.Decision)
	assert.Equal(t, 0.8, reviewResult.Score) // Default approve score
	assert.Contains(t, reviewResult.Reasoning, "Good security practice")
	
	mockModel.AssertExpectations(t)
}

func TestReviewerAgent_Review_RejectDecision(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	reviewer := NewReviewerAgent("reviewer-1", mockModel, config, mockSandbox, SpecializationSecurity)
	
	proposal := Proposal{
		ID:          "proposal-1",
		AgentID:     "lead-1",
		Type:        ProposalTypeFileChange,
		Description: "Add insecure authentication",
		Changes: []Change{
			{
				Type:        ChangeTypeUpdate,
				Path:        "auth.go",
				NewContent:  "func auth() { return true }",
				Description: "Always return true for auth",
			},
		},
	}
	
	// Mock model response for rejection
	reviewResponse := model.PromptOutput{
		Response: "REJECT: This authentication bypass is a serious security vulnerability.",
		Metadata: map[string]string{
			"confidence": "0.95",
			"decision":   "reject",
		},
	}
	
	mockModel.On("RunPrompt", mock.Anything, mock.Anything).Return(reviewResponse, nil)
	
	ctx := context.Background()
	reviewResult, err := reviewer.Review(ctx, proposal)
	
	assert.NoError(t, err)
	assert.NotNil(t, reviewResult)
	assert.Equal(t, DecisionReject, reviewResult.Decision)
	assert.Equal(t, 0.3, reviewResult.Score) // Reject score
	assert.Contains(t, reviewResult.Reasoning, "security vulnerability")
	
	mockModel.AssertExpectations(t)
}

func TestReviewerAgent_GetSpecialization(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	tests := []struct {
		name           string
		specialization string
	}{
		{"security specialist", SpecializationSecurity},
		{"performance specialist", SpecializationPerformance},
		{"architecture specialist", SpecializationArchitecture},
		{"testing specialist", SpecializationTesting},
		{"general reviewer", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewer := NewReviewerAgent("reviewer", mockModel, config, mockSandbox, tt.specialization)
			assert.Equal(t, tt.specialization, reviewer.specialization)
		})
	}
}

func TestReviewerAgent_SpecializationCapabilities(t *testing.T) {
	mockModel := &MockModel{}
	mockSandbox := &MockSandboxManager{}
	
	config := AgentConfig{
		Role:    RoleReviewer,
		Model:   "test-model",
		Enabled: true,
	}
	
	t.Run("security specialization adds security capability", func(t *testing.T) {
		reviewer := NewReviewerAgent("security-reviewer", mockModel, config, mockSandbox, SpecializationSecurity)
		capabilities := reviewer.GetCapabilities()
		
		// Should have security analysis capability
		assert.Contains(t, capabilities, CapabilitySecurityAnalysis)
		// Should also have base reviewer capabilities
		assert.Contains(t, capabilities, CapabilityCodeReview)
		assert.Contains(t, capabilities, CapabilityTesting)
	})
	
	t.Run("performance specialization adds performance capability", func(t *testing.T) {
		reviewer := NewReviewerAgent("perf-reviewer", mockModel, config, mockSandbox, SpecializationPerformance)
		capabilities := reviewer.GetCapabilities()
		
		// Should have performance analysis capability
		assert.Contains(t, capabilities, CapabilityPerformanceAnalysis)
		// Should also have base reviewer capabilities
		assert.Contains(t, capabilities, CapabilityCodeReview)
	})
	
	t.Run("architecture specialization adds architecture capability", func(t *testing.T) {
		reviewer := NewReviewerAgent("arch-reviewer", mockModel, config, mockSandbox, SpecializationArchitecture)
		capabilities := reviewer.GetCapabilities()
		
		// Should have architecture review capability
		assert.Contains(t, capabilities, CapabilityArchitectureReview)
		// Should also have base reviewer capabilities
		assert.Contains(t, capabilities, CapabilityCodeReview)
	})
}