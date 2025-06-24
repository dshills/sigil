// Package agent provides agent factory and management
package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/model"
	"github.com/dshills/sigil/internal/sandbox"
)

// Factory creates and manages agents
type Factory struct {
	sandbox sandbox.Manager
	config  OrchestrationConfig
}

// NewFactory creates a new agent factory
func NewFactory(sandbox sandbox.Manager, config OrchestrationConfig) *Factory {
	return &Factory{
		sandbox: sandbox,
		config:  config,
	}
}

// CreateAgent creates an agent based on configuration
func (f *Factory) CreateAgent(agentID string, agentConfig AgentConfig) (Agent, error) {
	// Parse model string to get provider and model
	provider, modelName, err := model.ParseModelString(agentConfig.Model)
	if err != nil {
		// If parsing fails, assume it's just a model name and use default provider
		provider = "openai" // Default provider
		modelName = agentConfig.Model
	}

	// Get model for the agent
	agentModel, err := model.GetModel(provider, modelName)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeModel, "CreateAgent",
			fmt.Sprintf("failed to get model %s:%s for agent %s", provider, modelName, agentID))
	}

	switch agentConfig.Role {
	case RoleLead:
		return NewLeadAgent(agentID, agentModel, agentConfig, f.sandbox), nil

	case RoleReviewer:
		specialization := agentConfig.Specialization
		if specialization == "" {
			specialization = "general"
		}
		return NewReviewerAgent(agentID, agentModel, agentConfig, f.sandbox, specialization), nil

	case RoleExpert:
		// Expert agents are specialized reviewers with domain expertise
		specialization := agentConfig.Specialization
		if specialization == "" {
			return nil, errors.New(errors.ErrorTypeConfig, "CreateAgent",
				"expert agents require specialization")
		}
		return NewReviewerAgent(agentID, agentModel, agentConfig, f.sandbox, specialization), nil

	default:
		return nil, errors.New(errors.ErrorTypeConfig, "CreateAgent",
			fmt.Sprintf("unsupported agent role: %s", agentConfig.Role))
	}
}

// CreateOrchestrator creates an orchestrator with configured agents
func (f *Factory) CreateOrchestrator() (*DefaultOrchestrator, error) {
	orchestrator := NewOrchestrator(f.config)

	// Create and register agents based on configuration
	for agentID, agentConfig := range f.config.AgentProfiles {
		if !agentConfig.Enabled {
			continue
		}

		agent, err := f.CreateAgent(agentID, agentConfig)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrorTypeConfig, "CreateOrchestrator",
				fmt.Sprintf("failed to create agent %s", agentID))
		}

		if err := orchestrator.RegisterAgent(agent); err != nil {
			return nil, errors.Wrap(err, errors.ErrorTypeConfig, "CreateOrchestrator",
				fmt.Sprintf("failed to register agent %s", agentID))
		}
	}

	return orchestrator, nil
}

// CreateTaskFromCommand creates a task from CLI command parameters
func (f *Factory) CreateTaskFromCommand(taskType TaskType, description string, files []string, requirements []string, constraints []Constraint) (*Task, error) {
	// Read file contents for context
	fileContexts := make([]FileContext, 0, len(files))
	for _, filePath := range files {
		// In a real implementation, you'd read the file content
		// For now, we'll create a placeholder
		fileContext := FileContext{
			Path:        filePath,
			Content:     "// File content would be read here",
			Language:    detectLanguage(filePath),
			Purpose:     "Target file for modification",
			IsTarget:    true,
			IsReference: false,
		}
		fileContexts = append(fileContexts, fileContext)
	}

	// Create project info (this could be enhanced to read from project files)
	projectInfo := ProjectInfo{
		Language:  "go", // Default, could be detected
		Framework: "",
		Version:   "",
		Style:     "standard",
	}

	task := &Task{
		ID:          fmt.Sprintf("task_%d", time.Now().Unix()),
		Type:        taskType,
		Description: description,
		Context: TaskContext{
			Files:        fileContexts,
			Requirements: requirements,
			ProjectInfo:  projectInfo,
		},
		Constraints: constraints,
		Priority:    PriorityMedium,
		CreatedAt:   time.Now(),
	}

	return task, nil
}

// detectLanguage detects programming language from file extension
func detectLanguage(filePath string) string {
	if strings.HasSuffix(filePath, ".go") {
		return "go"
	} else if strings.HasSuffix(filePath, ".js") || strings.HasSuffix(filePath, ".ts") {
		return "javascript"
	} else if strings.HasSuffix(filePath, ".py") {
		return "python"
	} else if strings.HasSuffix(filePath, ".java") {
		return "java"
	} else if strings.HasSuffix(filePath, ".cpp") || strings.HasSuffix(filePath, ".c") {
		return "c++"
	}
	return "text"
}

// CreateConstraintsFromFlags creates constraints from CLI flags
func (f *Factory) CreateConstraintsFromFlags(secure bool, fast bool, maintainable bool) []Constraint {
	var constraints []Constraint

	if secure {
		constraints = append(constraints, Constraint{
			Type:        ConstraintTypeSecurity,
			Description: "Ensure secure coding practices and avoid vulnerabilities",
			Severity:    SeverityError,
		})
	}

	if fast {
		constraints = append(constraints, Constraint{
			Type:        ConstraintTypePerformance,
			Description: "Optimize for performance and efficiency",
			Severity:    SeverityWarning,
		})
	}

	if maintainable {
		constraints = append(constraints, Constraint{
			Type:        ConstraintTypeStyle,
			Description: "Follow best practices for maintainable code",
			Severity:    SeverityWarning,
		})
	}

	return constraints
}

// ValidateConfig validates the orchestration configuration
func (f *Factory) ValidateConfig() error {
	if f.config.MaxAgents <= 0 {
		return errors.New(errors.ErrorTypeConfig, "ValidateConfig", "max_agents must be positive")
	}

	if f.config.ConsensusThreshold < 0.0 || f.config.ConsensusThreshold > 1.0 {
		return errors.New(errors.ErrorTypeConfig, "ValidateConfig", "consensus_threshold must be between 0.0 and 1.0")
	}

	if f.config.TaskTimeout <= 0 {
		return errors.New(errors.ErrorTypeConfig, "ValidateConfig", "task_timeout must be positive")
	}

	if f.config.ReviewTimeout <= 0 {
		return errors.New(errors.ErrorTypeConfig, "ValidateConfig", "review_timeout must be positive")
	}

	// Validate agent profiles
	leadAgents := 0
	for agentID, agentConfig := range f.config.AgentProfiles {
		if !agentConfig.Enabled {
			continue
		}

		if agentConfig.Model == "" {
			return errors.New(errors.ErrorTypeConfig, "ValidateConfig",
				fmt.Sprintf("agent %s missing model configuration", agentID))
		}

		if agentConfig.Role == RoleLead {
			leadAgents++
		}

		if agentConfig.Role == RoleExpert && agentConfig.Specialization == "" {
			return errors.New(errors.ErrorTypeConfig, "ValidateConfig",
				fmt.Sprintf("expert agent %s missing specialization", agentID))
		}
	}

	if leadAgents == 0 {
		return errors.New(errors.ErrorTypeConfig, "ValidateConfig", "at least one lead agent must be configured")
	}

	return nil
}

// GetRecommendedConfig returns a recommended configuration based on available models
func (f *Factory) GetRecommendedConfig() OrchestrationConfig {
	config := DefaultOrchestrationConfig()

	// Check available models and adjust configuration
	availableModels := model.ListModels()

	// Prefer Claude for lead agent if available
	claudeFound := false
	gptFound := false

	for _, modelName := range availableModels {
		if strings.Contains(strings.ToLower(modelName), "claude") {
			claudeFound = true
		}
		if strings.Contains(strings.ToLower(modelName), "gpt") {
			gptFound = true
		}
	}

	// Update agent profiles based on available models
	if claudeFound {
		if profile, exists := config.AgentProfiles["lead"]; exists {
			profile.Model = "claude-3-5-sonnet-20241022"
			config.AgentProfiles["lead"] = profile
		}
	}

	if gptFound {
		if profile, exists := config.AgentProfiles["reviewer"]; exists {
			profile.Model = "gpt-4"
			config.AgentProfiles["reviewer"] = profile
		}

		// Add specialized reviewers
		config.AgentProfiles["security_reviewer"] = AgentConfig{
			Role:           RoleReviewer,
			Model:          "gpt-4",
			Capabilities:   []Capability{CapabilityCodeReview, CapabilitySecurityAnalysis},
			Priority:       3,
			MaxConcurrency: 2,
			Specialization: "security",
			Enabled:        true,
		}

		config.AgentProfiles["performance_reviewer"] = AgentConfig{
			Role:           RoleReviewer,
			Model:          "gpt-4",
			Capabilities:   []Capability{CapabilityCodeReview, CapabilityPerformanceAnalysis},
			Priority:       3,
			MaxConcurrency: 2,
			Specialization: "performance",
			Enabled:        true,
		}
	}

	return config
}
