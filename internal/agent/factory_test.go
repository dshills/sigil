package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	mockSandbox := &MockSandboxManager{}
	config := DefaultOrchestrationConfig()

	factory := NewFactory(mockSandbox, config)

	assert.NotNil(t, factory)
	assert.Equal(t, mockSandbox, factory.sandbox)
	assert.Equal(t, config, factory.config)
}

func TestFactory_CreateLeadAgent(t *testing.T) {
	mockSandbox := &MockSandboxManager{}
	config := DefaultOrchestrationConfig()
	factory := NewFactory(mockSandbox, config)

	agentConfig := AgentConfig{
		Role:         RoleLead,
		Model:        "openai:gpt-4",
		Capabilities: []Capability{CapabilityCodeGeneration},
		Priority:     1,
		Enabled:      true,
	}

	// This will fail because we don't have real model providers
	// but we can test the factory logic up to that point
	agent, err := factory.CreateAgent("lead-1", agentConfig)
	
	// We expect an error because the model provider isn't registered
	assert.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "failed to get model")
}

func TestFactory_CreateReviewerAgent(t *testing.T) {
	mockSandbox := &MockSandboxManager{}
	config := DefaultOrchestrationConfig()
	factory := NewFactory(mockSandbox, config)

	agentConfig := AgentConfig{
		Role:           RoleReviewer,
		Model:          "anthropic:claude-3",
		Capabilities:   []Capability{CapabilityCodeReview},
		Priority:       2,
		Specialization: SpecializationSecurity,
		Enabled:        true,
	}

	agent, err := factory.CreateAgent("reviewer-1", agentConfig)
	
	// We expect an error because the model provider isn't registered
	assert.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "failed to get model")
}

func TestFactory_CreateAgentWithInvalidRole(t *testing.T) {
	mockSandbox := &MockSandboxManager{}
	config := DefaultOrchestrationConfig()
	factory := NewFactory(mockSandbox, config)

	agentConfig := AgentConfig{
		Role:    AgentRole("invalid"),
		Model:   "openai:gpt-4",
		Enabled: true,
	}

	agent, err := factory.CreateAgent("invalid-1", agentConfig)
	
	// Should fail on model lookup before reaching role validation
	assert.Error(t, err)
	assert.Nil(t, agent)
}

func TestFactory_ValidateConfig(t *testing.T) {
	mockSandbox := &MockSandboxManager{}
	config := DefaultOrchestrationConfig()
	factory := NewFactory(mockSandbox, config)

	t.Run("valid config", func(t *testing.T) {
		err := factory.ValidateConfig()
		assert.NoError(t, err)
	})

	t.Run("invalid max agents", func(t *testing.T) {
		invalidConfig := config
		invalidConfig.MaxAgents = 0
		invalidFactory := NewFactory(mockSandbox, invalidConfig)
		
		err := invalidFactory.ValidateConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_agents must be positive")
	})

	t.Run("invalid consensus threshold", func(t *testing.T) {
		invalidConfig := config
		invalidConfig.ConsensusThreshold = 1.5
		invalidFactory := NewFactory(mockSandbox, invalidConfig)
		
		err := invalidFactory.ValidateConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consensus_threshold must be between 0.0 and 1.0")
	})
}

func TestFactory_GetRecommendedConfig(t *testing.T) {
	mockSandbox := &MockSandboxManager{}
	config := DefaultOrchestrationConfig()
	factory := NewFactory(mockSandbox, config)

	recommendedConfig := factory.GetRecommendedConfig()
	
	assert.NotNil(t, recommendedConfig)
	assert.Equal(t, 5, recommendedConfig.MaxAgents)
	assert.Equal(t, 0.7, recommendedConfig.ConsensusThreshold)
	assert.Contains(t, recommendedConfig.AgentProfiles, "lead")
	assert.Contains(t, recommendedConfig.AgentProfiles, "reviewer")
}

func TestAgentConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config AgentConfig
		valid  bool
	}{
		{
			name: "valid lead config",
			config: AgentConfig{
				Role:         RoleLead,
				Model:        "openai:gpt-4",
				Capabilities: []Capability{CapabilityCodeGeneration},
				Priority:     1,
				Enabled:      true,
			},
			valid: true,
		},
		{
			name: "valid reviewer config",
			config: AgentConfig{
				Role:         RoleReviewer,
				Model:        "anthropic:claude-3",
				Capabilities: []Capability{CapabilityCodeReview},
				Priority:     2,
				Enabled:      true,
			},
			valid: true,
		},
		{
			name: "disabled agent",
			config: AgentConfig{
				Role:         RoleLead,
				Model:        "openai:gpt-4",
				Capabilities: []Capability{CapabilityCodeGeneration},
				Priority:     1,
				Enabled:      false,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that we can create the config struct
			assert.NotEmpty(t, tt.config.Role)
			assert.NotEmpty(t, tt.config.Model)
			
			if tt.valid {
				assert.True(t, len(tt.config.Capabilities) > 0 || !tt.config.Enabled)
			}
		})
	}
}

func TestOrchestrationConfig_Defaults(t *testing.T) {
	config := DefaultOrchestrationConfig()

	t.Run("basic settings", func(t *testing.T) {
		assert.Equal(t, 5, config.MaxAgents)
		assert.Equal(t, 0.7, config.ConsensusThreshold)
		assert.Equal(t, ResolutionVoting, config.ConflictResolution)
		assert.True(t, config.EnableParallelReview)
		assert.Equal(t, 3, config.MaxRetries)
	})

	t.Run("timeouts", func(t *testing.T) {
		assert.Equal(t, "10m0s", config.TaskTimeout.String())
		assert.Equal(t, "5m0s", config.ReviewTimeout.String())
	})

	t.Run("quality gate", func(t *testing.T) {
		assert.Equal(t, 0.8, config.QualityGate.MinConfidence)
		assert.Equal(t, 2, config.QualityGate.MinReviewers)
		assert.Equal(t, 4, config.QualityGate.MaxReviewers)
		assert.Contains(t, config.QualityGate.RequiredCapabilities, CapabilityCodeReview)
	})

	t.Run("agent profiles", func(t *testing.T) {
		require.Contains(t, config.AgentProfiles, "lead")
		require.Contains(t, config.AgentProfiles, "reviewer")

		leadProfile := config.AgentProfiles["lead"]
		assert.Equal(t, RoleLead, leadProfile.Role)
		assert.Equal(t, 1, leadProfile.Priority)
		assert.Equal(t, 1, leadProfile.MaxConcurrency)
		assert.True(t, leadProfile.Enabled)

		reviewerProfile := config.AgentProfiles["reviewer"]
		assert.Equal(t, RoleReviewer, reviewerProfile.Role)
		assert.Equal(t, 2, reviewerProfile.Priority)
		assert.Equal(t, 3, reviewerProfile.MaxConcurrency)
		assert.True(t, reviewerProfile.Enabled)
	})
}

func TestResolutionMethods(t *testing.T) {
	tests := []struct {
		name     string
		method   ResolutionMethod
		expected string
	}{
		{"voting method", ResolutionVoting, "voting"},
		{"expert rule method", ResolutionExpertRule, "expert_rule"},
		{"compromise method", ResolutionCompromise, "compromise"},
		{"arbitration method", ResolutionArbitration, "arbitration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.method))
		})
	}
}

func TestConflictTypes(t *testing.T) {
	tests := []struct {
		name     string
		conflict ConflictType
		expected string
	}{
		{"decision conflict", ConflictTypeDecision, "decision"},
		{"approach conflict", ConflictTypeApproach, "approach"},
		{"implementation conflict", ConflictTypeImplementation, "implementation"},
		{"priority conflict", ConflictTypePriority, "priority"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.conflict))
		})
	}
}

func TestConsensusDecisions(t *testing.T) {
	tests := []struct {
		name     string
		decision ConsensusDecision
		expected string
	}{
		{"approve consensus", ConsensusApprove, "approve"},
		{"reject consensus", ConsensusReject, "reject"},
		{"require changes consensus", ConsensusRequireChanges, "require_changes"},
		{"no consensus", ConsensusNoConsensus, "no_consensus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.decision))
		})
	}
}

func TestArtifactTypes(t *testing.T) {
	tests := []struct {
		name     string
		artifact ArtifactType
		expected string
	}{
		{"file artifact", ArtifactTypeFile, "file"},
		{"report artifact", ArtifactTypeReport, "report"},
		{"log artifact", ArtifactTypeLog, "log"},
		{"documentation artifact", ArtifactTypeDocumentation, "documentation"},
		{"test artifact", ArtifactTypeTest, "test"},
		{"configuration artifact", ArtifactTypeConfiguration, "configuration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.artifact))
		})
	}
}

func TestTestTypes(t *testing.T) {
	tests := []struct {
		name     string
		testType TestType
		expected string
	}{
		{"unit test", TestTypeUnit, "unit"},
		{"integration test", TestTypeIntegration, "integration"},
		{"lint test", TestTypeLint, "lint"},
		{"build test", TestTypeBuild, "build"},
		{"custom test", TestTypeCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.testType))
		})
	}
}

func TestTestStatuses(t *testing.T) {
	tests := []struct {
		name     string
		status   TestStatus
		expected string
	}{
		{"passed status", TestStatusPassed, "passed"},
		{"failed status", TestStatusFailed, "failed"},
		{"skipped status", TestStatusSkipped, "skipped"},
		{"error status", TestStatusError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}