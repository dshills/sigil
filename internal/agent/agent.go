// Package agent provides base agent implementation
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
	"github.com/dshills/sigil/internal/sandbox"
)

// BaseAgent provides common functionality for all agents
type BaseAgent struct {
	id           string
	role         AgentRole
	model        model.Model
	capabilities []Capability
	config       AgentConfig
	sandbox      sandbox.Manager
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(id string, role AgentRole, model model.Model, capabilities []Capability, config AgentConfig, sandbox sandbox.Manager) *BaseAgent {
	return &BaseAgent{
		id:           id,
		role:         role,
		model:        model,
		capabilities: capabilities,
		config:       config,
		sandbox:      sandbox,
	}
}

// GetID returns the agent's unique identifier
func (a *BaseAgent) GetID() string {
	return a.id
}

// GetRole returns the agent's role
func (a *BaseAgent) GetRole() AgentRole {
	return a.role
}

// GetCapabilities returns the agent's capabilities
func (a *BaseAgent) GetCapabilities() []Capability {
	return a.capabilities
}

// GetModel returns the underlying LLM model
func (a *BaseAgent) GetModel() model.Model {
	return a.model
}

// HasCapability checks if the agent has a specific capability
func (a *BaseAgent) HasCapability(capability Capability) bool {
	for _, cap := range a.capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// LeadAgent implements the lead agent responsible for primary task execution
type LeadAgent struct {
	*BaseAgent
}

// NewLeadAgent creates a new lead agent
func NewLeadAgent(id string, model model.Model, config AgentConfig, sandbox sandbox.Manager) *LeadAgent {
	capabilities := []Capability{
		CapabilityCodeGeneration,
		CapabilityRefactoring,
		CapabilityDocumentation,
	}

	baseAgent := NewBaseAgent(id, RoleLead, model, capabilities, config, sandbox)

	return &LeadAgent{
		BaseAgent: baseAgent,
	}
}

// Execute performs the primary task execution
func (a *LeadAgent) Execute(ctx context.Context, task Task) (*Result, error) {
	logger.Debug("lead agent executing task", "agent_id", a.id, "task_id", task.ID, "task_type", task.Type)

	startTime := time.Now()
	result := &Result{
		TaskID:    task.ID,
		AgentID:   a.id,
		Status:    StatusSuccess,
		Timestamp: startTime,
	}

	// Generate the system prompt based on task
	systemPrompt := a.generateSystemPrompt(task)

	// Generate the user prompt with context
	userPrompt := a.generateUserPrompt(task)

	// Create model request
	request := model.PromptInput{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    4000,
		Temperature:  0.1, // Lower temperature for more deterministic code generation
	}

	// Execute the model request
	response, err := a.model.RunPrompt(ctx, request)
	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, errors.Wrap(err, errors.ErrorTypeModel, "Execute", "model generation failed")
	}

	// Parse the response and create proposals
	proposals, reasoning, confidence := a.parseResponse(response.Response, task)

	result.Proposals = proposals
	result.Reasoning = reasoning
	result.Confidence = confidence
	result.Duration = time.Since(startTime)

	logger.Info("lead agent completed task", "agent_id", a.id, "task_id", task.ID,
		"status", result.Status, "proposals", len(proposals), "duration", result.Duration)

	return result, nil
}

// Review provides feedback on proposals (lead agents can also review)
func (a *LeadAgent) Review(ctx context.Context, proposal Proposal) (*ReviewResult, error) {
	logger.Debug("lead agent reviewing proposal", "agent_id", a.id, "proposal_id", proposal.ID)

	startTime := time.Now()

	// Generate review prompt
	systemPrompt := a.generateReviewSystemPrompt()
	userPrompt := a.generateReviewUserPrompt(proposal)

	request := model.PromptInput{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    2000,
		Temperature:  0.2,
	}

	response, err := a.model.RunPrompt(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeModel, "Review", "model generation failed")
	}

	// Parse review response
	reviewResult := a.parseReviewResponse(response.Response, proposal)

	reviewResult.ProposalID = proposal.ID
	reviewResult.ReviewerID = a.id
	reviewResult.Timestamp = startTime

	logger.Debug("lead agent completed review", "agent_id", a.id, "proposal_id", proposal.ID,
		"decision", reviewResult.Decision, "score", reviewResult.Score)

	return reviewResult, nil
}

// generateSystemPrompt creates the system prompt for task execution
func (a *LeadAgent) generateSystemPrompt(task Task) string {
	prompt := fmt.Sprintf(`You are a lead software engineering agent specialized in %s. Your role is to:

1. Analyze the given task and understand the requirements
2. Generate high-quality code solutions that follow best practices
3. Provide clear reasoning for your decisions
4. Create comprehensive proposals with proper change descriptions

Key capabilities:
- Code generation and modification
- Refactoring and optimization
- Documentation creation
- Architecture design

Task type: %s
Priority: %s

Guidelines:
- Write clean, maintainable, and well-documented code
- Follow established coding conventions and patterns
- Consider performance, security, and scalability
- Provide detailed explanations for complex changes
- Structure your response in a clear, parseable format

Response format:
Please structure your response as:

REASONING:
[Explain your approach and decisions]

CONFIDENCE: [0.0-1.0]

PROPOSALS:
[List of numbered proposals with changes]

For each proposal, include:
- Description of what the change does
- File path and content changes
- Rationale for the approach
- Any dependencies or considerations`,
		task.Context.ProjectInfo.Language,
		task.Type,
		task.Priority)

	// Add constraints if any
	if len(task.Constraints) > 0 {
		prompt += "\n\nConstraints to consider:\n"
		for _, constraint := range task.Constraints {
			prompt += fmt.Sprintf("- %s: %s (Severity: %s)\n", constraint.Type, constraint.Description, constraint.Severity)
		}
	}

	return prompt
}

// generateUserPrompt creates the user prompt with task context
func (a *LeadAgent) generateUserPrompt(task Task) string {
	prompt := fmt.Sprintf("Task: %s\n\nDescription: %s\n\n", task.Type, task.Description)

	// Add project context
	if task.Context.ProjectInfo.Language != "" {
		prompt += fmt.Sprintf("Project Language: %s\n", task.Context.ProjectInfo.Language)
	}
	if task.Context.ProjectInfo.Framework != "" {
		prompt += fmt.Sprintf("Framework: %s\n", task.Context.ProjectInfo.Framework)
	}

	// Add file context
	if len(task.Context.Files) > 0 {
		prompt += "\nRelevant Files:\n"
		for _, file := range task.Context.Files {
			prompt += fmt.Sprintf("\n--- %s ---\n", file.Path)
			if file.Purpose != "" {
				prompt += fmt.Sprintf("Purpose: %s\n", file.Purpose)
			}
			if file.IsTarget {
				prompt += "Target: This file should be modified\n"
			}
			if file.IsReference {
				prompt += "Reference: This file is for context only\n"
			}
			prompt += fmt.Sprintf("Content:\n%s\n", file.Content)
		}
	}

	// Add requirements
	if len(task.Context.Requirements) > 0 {
		prompt += "\nRequirements:\n"
		for i, req := range task.Context.Requirements {
			prompt += fmt.Sprintf("%d. %s\n", i+1, req)
		}
	}

	// Add examples if any
	if len(task.Context.Examples) > 0 {
		prompt += "\nExamples:\n"
		for i, example := range task.Context.Examples {
			prompt += fmt.Sprintf("\nExample %d: %s\n", i+1, example.Description)
			if example.Input != "" {
				prompt += fmt.Sprintf("Input:\n%s\n", example.Input)
			}
			if example.Output != "" {
				prompt += fmt.Sprintf("Expected Output:\n%s\n", example.Output)
			}
			if example.Explanation != "" {
				prompt += fmt.Sprintf("Explanation: %s\n", example.Explanation)
			}
		}
	}

	return prompt
}

// parseResponse parses the model response and extracts proposals
func (a *LeadAgent) parseResponse(content string, _ Task) ([]Proposal, string, float64) {
	// This is a simplified parser - in a real implementation, you'd want more robust parsing
	// For now, we'll create a basic proposal from the response

	proposal := Proposal{
		ID:          fmt.Sprintf("prop_%s_%d", a.id, time.Now().Unix()),
		AgentID:     a.id,
		Type:        ProposalTypeFileChange,
		Description: "Generated solution based on task requirements",
		Reasoning:   content, // Simplified - extract reasoning section
		Confidence:  0.8,     // Default confidence
		Impact: Impact{
			Scope:    ScopeLocal,
			Risk:     RiskLow,
			Benefits: []string{"Addresses task requirements"},
		},
		CreatedAt: time.Now(),
	}

	// In a real implementation, you'd parse the structured response to extract:
	// - Reasoning section
	// - Confidence score
	// - Individual changes
	// - Test cases
	// For now, we'll return a basic structure

	return []Proposal{proposal}, content, 0.8
}

// generateReviewSystemPrompt creates the system prompt for reviews
func (a *LeadAgent) generateReviewSystemPrompt() string {
	return `You are a lead software engineering agent performing code review. Your role is to:

1. Evaluate the quality and correctness of proposed changes
2. Check for adherence to best practices and coding standards
3. Identify potential issues, improvements, or risks
4. Provide constructive feedback and suggestions

Review criteria:
- Code quality and maintainability
- Performance implications
- Security considerations
- Compatibility and breaking changes
- Test coverage and validation
- Documentation completeness

Response format:
Please structure your review as:

DECISION: [approve|request_changes|reject|needs_more_info]
SCORE: [0.0-1.0]
CONFIDENCE: [0.0-1.0]

COMMENTS:
[Numbered list of specific comments]

SUGGESTIONS:
[Any improvement suggestions]

REASONING:
[Overall assessment and rationale]`
}

// generateReviewUserPrompt creates the user prompt for proposal review
func (a *LeadAgent) generateReviewUserPrompt(proposal Proposal) string {
	prompt := "Please review the following proposal:\n\n"
	prompt += fmt.Sprintf("Proposal ID: %s\n", proposal.ID)
	prompt += fmt.Sprintf("Type: %s\n", proposal.Type)
	prompt += fmt.Sprintf("Description: %s\n\n", proposal.Description)

	if len(proposal.Changes) > 0 {
		prompt += "Changes:\n"
		for i, change := range proposal.Changes {
			prompt += fmt.Sprintf("\n%d. %s (Type: %s)\n", i+1, change.Description, change.Type)
			prompt += fmt.Sprintf("   Path: %s\n", change.Path)
			if change.NewContent != "" {
				prompt += fmt.Sprintf("   New Content:\n%s\n", change.NewContent)
			}
		}
	}

	prompt += fmt.Sprintf("\nProposal Reasoning:\n%s\n", proposal.Reasoning)
	prompt += fmt.Sprintf("Agent Confidence: %.2f\n", proposal.Confidence)
	prompt += fmt.Sprintf("Estimated Impact: %s (Risk: %s)\n", proposal.Impact.Scope, proposal.Impact.Risk)

	return prompt
}

// parseReviewResponse parses the review response
func (a *LeadAgent) parseReviewResponse(content string, _ Proposal) *ReviewResult {
	// Simplified parsing - in a real implementation, you'd parse the structured response
	result := &ReviewResult{
		Decision:   DecisionApprove, // Default decision
		Score:      0.8,             // Default score
		Confidence: 0.8,             // Default confidence
		Reasoning:  content,
		Comments: []ReviewComment{
			{
				Type:     CommentTypeGeneral,
				Severity: SeverityInfo,
				Message:  "Review completed",
			},
		},
	}

	return result
}
