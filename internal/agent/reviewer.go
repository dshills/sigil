// Package agent provides reviewer agent implementation
package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
	"github.com/dshills/sigil/internal/sandbox"
)

// ReviewerAgent implements specialized agents for code review
type ReviewerAgent struct {
	*BaseAgent
	specialization string
}

// NewReviewerAgent creates a new reviewer agent
func NewReviewerAgent(id string, model model.Model, config AgentConfig, sandbox sandbox.Manager, specialization string) *ReviewerAgent {
	capabilities := []Capability{
		CapabilityCodeReview,
		CapabilityTesting,
		CapabilitySecurityAnalysis,
	}

	// Add specialized capabilities based on specialization
	switch specialization {
	case SpecializationSecurity:
		capabilities = append(capabilities, CapabilitySecurityAnalysis)
	case SpecializationPerformance:
		capabilities = append(capabilities, CapabilityPerformanceAnalysis)
	case SpecializationArchitecture:
		capabilities = append(capabilities, CapabilityArchitectureReview)
	}

	baseAgent := NewBaseAgent(id, RoleReviewer, model, capabilities, config, sandbox)

	return &ReviewerAgent{
		BaseAgent:      baseAgent,
		specialization: specialization,
	}
}

// Execute performs review-focused task execution
func (a *ReviewerAgent) Execute(ctx context.Context, task Task) (*Result, error) {
	logger.Debug("reviewer agent executing task", "agent_id", a.id, "task_id", task.ID, "specialization", a.specialization)

	startTime := time.Now()
	result := &Result{
		TaskID:    task.ID,
		AgentID:   a.id,
		Status:    StatusSuccess,
		Timestamp: startTime,
	}

	// Reviewer agents primarily focus on analysis and validation tasks
	switch task.Type {
	case TaskTypeReview:
		return a.executeReviewTask(ctx, task, result)
	case TaskTypeAnalyze:
		return a.executeAnalysisTask(ctx, task, result)
	case TaskTypeTest:
		return a.executeTestTask(ctx, task, result)
	case TaskTypeEdit, TaskTypeGenerate, TaskTypeRefactor, TaskTypeDocument, TaskTypeOptimize:
		result.Status = StatusFailed
		result.Error = fmt.Sprintf("task type %s not supported by reviewer agent, use lead agent instead", task.Type)
		result.Duration = time.Since(startTime)
		return result, errors.New(errors.ErrorTypeInput, "Execute", result.Error)
	default:
		result.Status = StatusFailed
		result.Error = fmt.Sprintf("unsupported task type for reviewer agent: %s", task.Type)
		result.Duration = time.Since(startTime)
		return result, errors.New(errors.ErrorTypeInput, "Execute", result.Error)
	}
}

// Review provides specialized review capabilities
func (a *ReviewerAgent) Review(ctx context.Context, proposal Proposal) (*ReviewResult, error) {
	logger.Debug("reviewer agent reviewing proposal", "agent_id", a.id, "proposal_id", proposal.ID, "specialization", a.specialization)

	startTime := time.Now()

	// Generate specialized review prompt based on agent's specialization
	systemPrompt := a.generateSpecializedReviewPrompt()
	userPrompt := a.generateDetailedReviewPrompt(proposal)

	request := model.PromptInput{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    3000,
		Temperature:  0.1, // Low temperature for consistent reviews
	}

	response, err := a.model.RunPrompt(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeModel, "Review", "model generation failed")
	}

	// Parse the specialized review response
	reviewResult := a.parseSpecializedReviewResponse(response.Response, proposal)

	reviewResult.ProposalID = proposal.ID
	reviewResult.ReviewerID = a.id
	reviewResult.Timestamp = startTime

	// If sandbox is available, run validation tests
	if a.sandbox != nil {
		testResults := a.runValidationTests(ctx, proposal)
		reviewResult.Tests = testResults
	}

	logger.Info("reviewer agent completed review", "agent_id", a.id, "proposal_id", proposal.ID,
		"decision", reviewResult.Decision, "score", reviewResult.Score, "specialization", a.specialization)

	return reviewResult, nil
}

// executeReviewTask executes a review-specific task
func (a *ReviewerAgent) executeReviewTask(ctx context.Context, task Task, result *Result) (*Result, error) {
	// Generate review analysis
	systemPrompt := fmt.Sprintf(`You are a %s reviewer agent. Analyze the provided code and generate a comprehensive review report.

Focus areas based on your specialization:
%s

Provide detailed analysis including:
- Issues found
- Best practice violations
- Security vulnerabilities (if applicable)
- Performance concerns (if applicable)
- Recommendations for improvement`, a.specialization, a.getSpecializationDescription())

	userPrompt := a.generateTaskUserPrompt(task)

	request := model.PromptInput{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    4000,
		Temperature:  0.2,
	}

	response, err := a.model.RunPrompt(ctx, request)
	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		result.Duration = time.Since(result.Timestamp)
		return result, errors.Wrap(err, errors.ErrorTypeModel, "executeReviewTask", "model generation failed")
	}

	// Create analysis artifact
	artifact := Artifact{
		Name:      fmt.Sprintf("review_analysis_%s", a.specialization),
		Type:      ArtifactTypeReport,
		Content:   response.Response,
		CreatedAt: time.Now(),
		Metadata: map[string]string{
			"specialization": a.specialization,
			"agent_id":       a.id,
		},
	}

	result.Artifacts = []Artifact{artifact}
	result.Reasoning = fmt.Sprintf("Completed %s review analysis", a.specialization)
	result.Confidence = 0.9
	result.Duration = time.Since(result.Timestamp)

	return result, nil
}

// executeAnalysisTask executes an analysis task
func (a *ReviewerAgent) executeAnalysisTask(ctx context.Context, task Task, result *Result) (*Result, error) {
	// Similar to review task but focused on deeper analysis
	return a.executeReviewTask(ctx, task, result)
}

// executeTestTask executes a testing task
func (a *ReviewerAgent) executeTestTask(ctx context.Context, task Task, result *Result) (*Result, error) {
	if a.specialization != SpecializationTesting && !a.HasCapability(CapabilityTesting) {
		result.Status = StatusFailed
		result.Error = "agent not specialized for testing tasks"
		result.Duration = time.Since(result.Timestamp)
		return result, errors.New(errors.ErrorTypeInput, "executeTestTask", result.Error)
	}

	// Generate test cases and validation
	systemPrompt := `You are a testing specialist reviewer agent. Your task is to:
1. Generate comprehensive test cases for the provided code
2. Identify testing gaps and coverage issues
3. Suggest testing strategies and frameworks
4. Validate existing tests for completeness and quality

Focus on:
- Unit test coverage
- Integration test requirements
- Edge cases and error handling
- Performance testing needs
- Security testing considerations`

	userPrompt := a.generateTaskUserPrompt(task)

	request := model.PromptInput{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    4000,
		Temperature:  0.3,
	}

	response, err := a.model.RunPrompt(ctx, request)
	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		result.Duration = time.Since(result.Timestamp)
		return result, errors.Wrap(err, errors.ErrorTypeModel, "executeTestTask", "model generation failed")
	}

	// Parse response to extract test cases
	testCases := a.parseTestCases(response.Response)

	// Create test proposal
	proposal := Proposal{
		ID:          fmt.Sprintf("test_prop_%s_%d", a.id, time.Now().Unix()),
		AgentID:     a.id,
		Type:        ProposalTypeFileCreation,
		Description: "Generated test cases and testing strategy",
		Reasoning:   response.Response,
		Confidence:  0.85,
		Impact: Impact{
			Scope: ScopeModule,
			Risk:  RiskLow,
			Benefits: []string{
				"Improved test coverage",
				"Better error handling validation",
				"Reduced regression risk",
			},
		},
		Tests:     testCases,
		CreatedAt: time.Now(),
	}

	result.Proposals = []Proposal{proposal}
	result.Reasoning = "Generated comprehensive testing strategy"
	result.Confidence = 0.85
	result.Duration = time.Since(result.Timestamp)

	return result, nil
}

// generateSpecializedReviewPrompt creates a review prompt based on specialization
func (a *ReviewerAgent) generateSpecializedReviewPrompt() string {
	basePrompt := `You are a specialized code reviewer with expertise in %s. Your role is to provide thorough, expert-level review focused on your area of specialization.

%s

Review Guidelines:
- Be thorough and detail-oriented
- Focus on issues within your area of expertise
- Provide specific, actionable feedback
- Suggest concrete improvements
- Rate the overall quality within your domain
- Consider industry best practices and standards

Response Format:
DECISION: [approve|request_changes|reject|needs_more_info]
SCORE: [0.0-1.0 - quality score for your specialization area]
CONFIDENCE: [0.0-1.0 - your confidence in this review]

SPECIALIZED_ANALYSIS:
[Your expert analysis focused on %s]

ISSUES_FOUND:
[List specific issues with severity levels]

RECOMMENDATIONS:
[Concrete improvement suggestions]

REASONING:
[Overall assessment and rationale]`

	specialization := a.specialization
	description := a.getSpecializationDescription()

	return fmt.Sprintf(basePrompt, specialization, description, specialization)
}

// generateDetailedReviewPrompt creates a detailed review prompt
func (a *ReviewerAgent) generateDetailedReviewPrompt(proposal Proposal) string {
	prompt := fmt.Sprintf("Please perform a detailed %s review of the following proposal:\n\n", a.specialization)
	prompt += fmt.Sprintf("Proposal ID: %s\n", proposal.ID)
	prompt += fmt.Sprintf("Type: %s\n", proposal.Type)
	prompt += fmt.Sprintf("Description: %s\n\n", proposal.Description)

	if len(proposal.Changes) > 0 {
		prompt += "Changes to Review:\n"
		for i, change := range proposal.Changes {
			prompt += fmt.Sprintf("\n%d. File: %s\n", i+1, change.Path)
			prompt += fmt.Sprintf("   Type: %s\n", change.Type)
			prompt += fmt.Sprintf("   Description: %s\n", change.Description)

			if change.OldContent != "" && change.NewContent != "" {
				prompt += "   Diff:\n"
				prompt += fmt.Sprintf("   Old:\n%s\n", change.OldContent)
				prompt += fmt.Sprintf("   New:\n%s\n", change.NewContent)
			} else if change.NewContent != "" {
				prompt += fmt.Sprintf("   Content:\n%s\n", change.NewContent)
			}
		}
	}

	prompt += fmt.Sprintf("\nOriginal Reasoning: %s\n", proposal.Reasoning)
	prompt += fmt.Sprintf("Confidence: %.2f\n", proposal.Confidence)
	prompt += fmt.Sprintf("Impact Assessment: %s (Risk: %s)\n", proposal.Impact.Scope, proposal.Impact.Risk)

	// Add specialization-specific focus areas
	prompt += fmt.Sprintf("\nFocus your %s review on:\n", a.specialization)
	prompt += strings.Join(a.getSpecializationFocusAreas(), "\n- ")

	return prompt
}

// generateTaskUserPrompt creates a user prompt for task execution
func (a *ReviewerAgent) generateTaskUserPrompt(task Task) string {
	prompt := fmt.Sprintf("Task: %s\nDescription: %s\n\n", task.Type, task.Description)

	// Add file context
	if len(task.Context.Files) > 0 {
		prompt += "Files to Analyze:\n"
		for _, file := range task.Context.Files {
			prompt += fmt.Sprintf("\n--- %s ---\n", file.Path)
			if file.Language != "" {
				prompt += fmt.Sprintf("Language: %s\n", file.Language)
			}
			if file.Purpose != "" {
				prompt += fmt.Sprintf("Purpose: %s\n", file.Purpose)
			}
			prompt += fmt.Sprintf("Content:\n%s\n", file.Content)
		}
	}

	return prompt
}

// getSpecializationDescription returns a description of the agent's specialization
func (a *ReviewerAgent) getSpecializationDescription() string {
	switch a.specialization {
	case SpecializationSecurity:
		return `Security Review Focus:
- Vulnerability assessment
- Input validation and sanitization
- Authentication and authorization
- Data protection and privacy
- Secure coding practices
- Common security anti-patterns
- Dependency security analysis`
	case SpecializationPerformance:
		return `Performance Review Focus:
- Algorithm efficiency and complexity
- Memory usage optimization
- I/O operations and bottlenecks
- Caching strategies
- Database query optimization
- Resource management
- Scalability considerations`
	case SpecializationArchitecture:
		return `Architecture Review Focus:
- Design patterns and principles
- Separation of concerns
- Modularity and coupling
- Extensibility and maintainability
- API design and contracts
- Data flow and dependencies
- System boundaries and interfaces`
	case SpecializationTesting:
		return `Testing Review Focus:
- Test coverage and completeness
- Test quality and maintainability
- Testing strategies and approaches
- Mock and stub usage
- Integration test design
- Edge case coverage
- Test automation and CI/CD`
	default:
		return `General Review Focus:
- Code quality and readability
- Best practice adherence
- Error handling
- Documentation completeness
- Maintainability concerns`
	}
}

// getSpecializationFocusAreas returns focus areas for the specialization
func (a *ReviewerAgent) getSpecializationFocusAreas() []string {
	switch a.specialization {
	case "security":
		return []string{
			"Input validation and sanitization",
			"Authentication and authorization mechanisms",
			"Data encryption and protection",
			"SQL injection and XSS vulnerabilities",
			"Secure API design",
			"Dependency vulnerabilities",
		}
	case "performance":
		return []string{
			"Algorithm complexity and efficiency",
			"Memory usage and garbage collection",
			"I/O operations and blocking calls",
			"Caching strategies",
			"Database query performance",
			"Resource utilization",
		}
	case "architecture":
		return []string{
			"Design pattern usage",
			"Separation of concerns",
			"Module dependencies and coupling",
			"API design and contracts",
			"Extensibility and maintainability",
			"System boundaries",
		}
	case "testing":
		return []string{
			"Test coverage and completeness",
			"Test case quality and maintainability",
			"Integration test design",
			"Edge case coverage",
			"Mock and stub usage",
			"Test automation",
		}
	default:
		return []string{
			"Code quality and readability",
			"Best practice adherence",
			"Error handling",
			"Documentation",
		}
	}
}

// parseSpecializedReviewResponse parses the review response
func (a *ReviewerAgent) parseSpecializedReviewResponse(content string, _ Proposal) *ReviewResult {
	// Simplified parsing - extract key sections
	result := &ReviewResult{
		Decision:    DecisionApprove, // Default
		Score:       0.8,
		Confidence:  0.9,
		Reasoning:   content,
		Comments:    []ReviewComment{},
		Suggestions: []Suggestion{},
	}

	// Parse decision
	if strings.Contains(strings.ToLower(content), "request_changes") {
		result.Decision = DecisionRequestChanges
		result.Score = 0.6
	} else if strings.Contains(strings.ToLower(content), "reject") {
		result.Decision = DecisionReject
		result.Score = 0.3
	} else if strings.Contains(strings.ToLower(content), "needs_more_info") {
		result.Decision = DecisionNeedsMoreInfo
		result.Score = 0.5
	}

	// Extract issues and create comments
	if strings.Contains(strings.ToLower(content), "security") && a.specialization == "security" {
		result.Comments = append(result.Comments, ReviewComment{
			Type:     CommentTypeSecurity,
			Severity: SeverityWarning,
			Message:  "Security review completed with specialized analysis",
		})
	}

	// Add specialization metadata
	if result.Metadata == nil {
		result.Metadata = make(map[string]string)
	}
	result.Metadata["specialization"] = a.specialization
	result.Metadata["agent_type"] = "reviewer"

	return result
}

// runValidationTests runs validation tests using the sandbox
func (a *ReviewerAgent) runValidationTests(ctx context.Context, proposal Proposal) []TestResult {
	if len(proposal.Tests) == 0 {
		return nil
	}

	results := make([]TestResult, 0, len(proposal.Tests))
	for _, testCase := range proposal.Tests {
		startTime := time.Now()

		// Create execution request for the test
		request := sandbox.ExecutionRequest{
			ID:   fmt.Sprintf("test_%s_%d", a.id, time.Now().Unix()),
			Type: "validation_test",
			ValidationSteps: []sandbox.ValidationStep{
				{
					Name:     testCase.Name,
					Command:  testCase.Command,
					Args:     testCase.Args,
					Required: true,
				},
			},
		}

		// Execute in sandbox
		response, err := a.sandbox.ExecuteCode(ctx, request)

		result := TestResult{
			TestCase: testCase,
			Duration: time.Since(startTime),
		}

		if err != nil {
			result.Status = TestStatusError
			result.Error = err.Error()
		} else if response.Success() {
			result.Status = TestStatusPassed
			if len(response.Results) > 0 {
				result.Output = response.Results[0].Output
			}
		} else {
			result.Status = TestStatusFailed
			if len(response.Results) > 0 {
				result.Error = response.Results[0].Error
				result.Output = response.Results[0].Output
			}
		}

		results = append(results, result)
	}

	return results
}

// parseTestCases parses test cases from generated content
func (a *ReviewerAgent) parseTestCases(_ string) []TestCase {
	// Simplified test case extraction
	// In a real implementation, you'd parse structured test case definitions

	testCases := []TestCase{
		{
			Name:        "Basic functionality test",
			Description: "Test basic functionality",
			Type:        TestTypeUnit,
			Command:     "go",
			Args:        []string{"test", "-v"},
			Timeout:     30 * time.Second,
		},
	}

	// Extract additional test cases from content if structured properly
	// This would involve parsing test case definitions from the response

	return testCases
}
