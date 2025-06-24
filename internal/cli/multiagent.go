// Package cli provides multi-agent command functionality
package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/agent"
	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/git"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/sandbox"
	"github.com/spf13/cobra"
)

// MultiAgentCommand implements multi-agent operations
type MultiAgentCommand struct {
	*BaseCommand
	UseMultiAgent bool
	EnableReview  bool
	MaxAgents     int
	Reviewers     []string
	TaskType      string
	Secure        bool
	Fast          bool
	Maintainable  bool
}

// NewMultiAgentCommand creates a new multi-agent command
func NewMultiAgentCommand() *MultiAgentCommand {
	return &MultiAgentCommand{
		BaseCommand: NewBaseCommand(
			"multi",
			"Execute tasks using multiple AI agents",
			`Execute complex tasks using a coordinated team of AI agents.
This command creates a lead agent to perform the primary task and 
reviewer agents to validate and improve the results through consensus.

Examples:
  sigil multi edit "Add error handling" --file main.go --review
  sigil multi generate "Create unit tests" --dir src/ --reviewers security,performance
  sigil multi refactor "Optimize database queries" --secure --fast
  sigil multi analyze "Review code quality" --file *.go --maintainable`,
		),
		UseMultiAgent: true,
		EnableReview:  true,
		MaxAgents:     5,
	}
}

// Execute runs the multi-agent command
func (c *MultiAgentCommand) Execute(ctx context.Context, args []string) error {
	start := time.Now()

	// Validate arguments
	if len(args) == 0 {
		return errors.New(errors.ErrorTypeInput, "Execute", "task description is required")
	}

	if c.TaskType == "" {
		return errors.New(errors.ErrorTypeInput, "Execute", "task type is required (edit, generate, analyze, etc.)")
	}

	taskDescription := strings.Join(args, " ")

	// Run pre-checks
	if err := c.RunPreChecks(); err != nil {
		return err
	}

	logger.Info("starting multi-agent task", "task_type", c.TaskType, "description", taskDescription)

	// Get input context
	inputHandler := NewInputHandler(c.GetCommonFlags())
	inputCtx, err := inputHandler.GetInput()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInput, "Execute", "failed to get input")
	}

	// Create sandbox manager
	repo, err := git.NewRepository(".")
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "Execute", "failed to open repository")
	}

	sandboxManager, err := sandbox.NewManager(repo)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeConfig, "Execute", "failed to create sandbox manager")
	}
	defer sandboxManager.Cleanup()

	// Create agent factory and orchestrator
	agentConfig := c.getAgentConfig()
	factory := agent.NewFactory(sandboxManager, agentConfig)

	if err := factory.ValidateConfig(); err != nil {
		return errors.Wrap(err, errors.ErrorTypeConfig, "Execute", "invalid agent configuration")
	}

	orchestrator, err := factory.CreateOrchestrator()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeConfig, "Execute", "failed to create orchestrator")
	}

	// Create task
	task, err := c.createTask(factory, taskDescription, inputCtx)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInput, "Execute", "failed to create task")
	}

	logger.Debug("created task", "id", task.ID, "type", task.Type, "files", len(task.Context.Files))

	// Execute task with orchestration
	result, err := orchestrator.ExecuteTask(ctx, *task)
	if err != nil {
		duration := time.Since(start)
		c.handleError(err, duration)
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "task execution failed")
	}

	// Handle results
	duration := time.Since(start)
	if err := c.handleResults(result, inputCtx, duration); err != nil {
		return errors.Wrap(err, errors.ErrorTypeOutput, "Execute", "failed to handle results")
	}

	logger.Info("multi-agent task completed", "task_id", task.ID, "status", result.Status,
		"duration", duration, "agents", len(orchestrator.GetAgents()))

	return nil
}

// createTask creates a task from command parameters
func (c *MultiAgentCommand) createTask(factory *agent.Factory, description string, inputCtx *CommandContext) (*agent.Task, error) {
	// Convert task type string to enum
	var taskType agent.TaskType
	switch strings.ToLower(c.TaskType) {
	case "edit", "modify":
		taskType = agent.TaskTypeEdit
	case "generate", "create":
		taskType = agent.TaskTypeGenerate
	case "refactor":
		taskType = agent.TaskTypeRefactor
	case "document", "doc":
		taskType = agent.TaskTypeDocument
	case "test":
		taskType = agent.TaskTypeTest
	case "review":
		taskType = agent.TaskTypeReview
	case "optimize":
		taskType = agent.TaskTypeOptimize
	case "analyze":
		taskType = agent.TaskTypeAnalyze
	default:
		return nil, errors.New(errors.ErrorTypeInput, "createTask",
			fmt.Sprintf("unsupported task type: %s", c.TaskType))
	}

	// Get file paths
	var filePaths []string
	for _, file := range inputCtx.Files {
		filePaths = append(filePaths, file.Path)
	}

	// Create constraints from flags
	constraints := factory.CreateConstraintsFromFlags(c.Secure, c.Fast, c.Maintainable)

	// Create requirements
	requirements := []string{description}
	if c.EnableReview {
		requirements = append(requirements, "Code must pass review by specialized agents")
	}

	task, err := factory.CreateTaskFromCommand(taskType, description, filePaths, requirements, constraints)
	if err != nil {
		return nil, err
	}

	// Add file contents to task context
	for i, inputFile := range inputCtx.Files {
		if i < len(task.Context.Files) {
			task.Context.Files[i].Content = inputFile.Content
			// Language is already detected in CreateTaskFromCommand from path
		}
	}

	return task, nil
}

// getAgentConfig creates agent configuration based on command flags
func (c *MultiAgentCommand) getAgentConfig() agent.OrchestrationConfig {
	config := agent.DefaultOrchestrationConfig()

	// Adjust based on command flags
	if c.MaxAgents > 0 {
		config.MaxAgents = c.MaxAgents
	}

	config.EnableParallelReview = c.EnableReview

	// Configure specialized reviewers based on flags
	if c.Secure {
		config.AgentProfiles["security_reviewer"] = agent.AgentConfig{
			Role:           agent.RoleReviewer,
			Model:          "gpt-4",
			Capabilities:   []agent.Capability{agent.CapabilityCodeReview, agent.CapabilitySecurityAnalysis},
			Priority:       2,
			MaxConcurrency: 1,
			Specialization: "security",
			Enabled:        true,
		}
	}

	if c.Fast {
		config.AgentProfiles["performance_reviewer"] = agent.AgentConfig{
			Role:           agent.RoleReviewer,
			Model:          "gpt-4",
			Capabilities:   []agent.Capability{agent.CapabilityCodeReview, agent.CapabilityPerformanceAnalysis},
			Priority:       2,
			MaxConcurrency: 1,
			Specialization: "performance",
			Enabled:        true,
		}
	}

	if c.Maintainable {
		config.AgentProfiles["architecture_reviewer"] = agent.AgentConfig{
			Role:           agent.RoleReviewer,
			Model:          "gpt-4",
			Capabilities:   []agent.Capability{agent.CapabilityCodeReview, agent.CapabilityArchitectureReview},
			Priority:       2,
			MaxConcurrency: 1,
			Specialization: "architecture",
			Enabled:        true,
		}
	}

	// Add specific reviewers if requested
	for _, reviewer := range c.Reviewers {
		reviewerID := fmt.Sprintf("%s_reviewer", reviewer)
		config.AgentProfiles[reviewerID] = agent.AgentConfig{
			Role:           agent.RoleReviewer,
			Model:          "gpt-4",
			Capabilities:   []agent.Capability{agent.CapabilityCodeReview},
			Priority:       3,
			MaxConcurrency: 1,
			Specialization: reviewer,
			Enabled:        true,
		}
	}

	return config
}

// handleResults processes and outputs the orchestration results
func (c *MultiAgentCommand) handleResults(result *agent.OrchestrationResult, inputCtx *CommandContext, duration time.Duration) error {
	output := &CommandOutput{
		Command:   "multi",
		Success:   result.Status == agent.StatusSuccess,
		Duration:  duration,
		Timestamp: time.Now(),
	}

	// Add error if task failed
	if result.Status != agent.StatusSuccess {
		output.Success = false
		if result.FinalResult != nil && result.FinalResult.Error != "" {
			output.Error = result.FinalResult.Error
		}
	}

	// Build response content
	var responseBuilder strings.Builder

	responseBuilder.WriteString(fmt.Sprintf("# Multi-Agent Task Results\n\n"))
	responseBuilder.WriteString(fmt.Sprintf("**Task ID:** %s\n", result.TaskID))
	responseBuilder.WriteString(fmt.Sprintf("**Status:** %s\n", result.Status))
	responseBuilder.WriteString(fmt.Sprintf("**Lead Agent:** %s\n", result.LeadAgent))
	responseBuilder.WriteString(fmt.Sprintf("**Duration:** %s\n\n", duration))

	// Add final result if available
	if result.FinalResult != nil {
		responseBuilder.WriteString("## Final Result\n\n")
		responseBuilder.WriteString(result.FinalResult.Reasoning)
		responseBuilder.WriteString("\n\n")

		if len(result.FinalResult.Proposals) > 0 {
			responseBuilder.WriteString("### Proposals\n\n")
			for i, proposal := range result.FinalResult.Proposals {
				responseBuilder.WriteString(fmt.Sprintf("%d. **%s** (Confidence: %.2f)\n",
					i+1, proposal.Description, proposal.Confidence))
				responseBuilder.WriteString(fmt.Sprintf("   %s\n\n", proposal.Reasoning))

				// Show changes
				if len(proposal.Changes) > 0 {
					responseBuilder.WriteString("   **Changes:**\n")
					for _, change := range proposal.Changes {
						responseBuilder.WriteString(fmt.Sprintf("   - %s: %s\n", change.Path, change.Description))
					}
					responseBuilder.WriteString("\n")
				}
			}
		}
	}

	// Add consensus information
	if result.Consensus != nil {
		responseBuilder.WriteString("## Review Consensus\n\n")
		responseBuilder.WriteString(fmt.Sprintf("**Decision:** %s\n", result.Consensus.Decision))
		responseBuilder.WriteString(fmt.Sprintf("**Score:** %.2f\n", result.Consensus.Score))
		responseBuilder.WriteString(fmt.Sprintf("**Reviewers:** %d\n\n", len(result.Consensus.Reviews)))

		if len(result.Consensus.Reviews) > 0 {
			responseBuilder.WriteString("### Review Details\n\n")
			for i, review := range result.Consensus.Reviews {
				responseBuilder.WriteString(fmt.Sprintf("**Reviewer %d** (%s)\n", i+1, review.ReviewerID))
				responseBuilder.WriteString(fmt.Sprintf("- Decision: %s\n", review.Decision))
				responseBuilder.WriteString(fmt.Sprintf("- Score: %.2f\n", review.Score))
				responseBuilder.WriteString(fmt.Sprintf("- Confidence: %.2f\n", review.Confidence))

				if len(review.Comments) > 0 {
					responseBuilder.WriteString("- Comments:\n")
					for _, comment := range review.Comments {
						responseBuilder.WriteString(fmt.Sprintf("  - %s: %s\n", comment.Type, comment.Message))
					}
				}
				responseBuilder.WriteString("\n")
			}
		}

		// Show conflicts if any
		if len(result.Consensus.Conflicts) > 0 {
			responseBuilder.WriteString("### Conflicts\n\n")
			for _, conflict := range result.Consensus.Conflicts {
				responseBuilder.WriteString(fmt.Sprintf("- **%s**: %s\n", conflict.Type, conflict.Description))
			}
			responseBuilder.WriteString("\n")
		}

		// Show resolution if available
		if result.Consensus.Resolution != nil {
			responseBuilder.WriteString("### Conflict Resolution\n\n")
			responseBuilder.WriteString(fmt.Sprintf("**Method:** %s\n", result.Consensus.Resolution.Method))
			responseBuilder.WriteString(fmt.Sprintf("**Description:** %s\n", result.Consensus.Resolution.Description))
			responseBuilder.WriteString(fmt.Sprintf("**Rationale:** %s\n\n", result.Consensus.Resolution.Rationale))
		}
	}

	output.Content = responseBuilder.String()

	// Write output
	outputHandler := NewOutputHandler(c.GetCommonFlags())
	return outputHandler.WriteOutput(output)
}

// handleError handles execution errors
func (c *MultiAgentCommand) handleError(err error, duration time.Duration) {
	output := &CommandOutput{
		Command:   "multi",
		Success:   false,
		Duration:  duration,
		Timestamp: time.Now(),
		Error:     err.Error(),
		Content:   fmt.Sprintf("Multi-agent task failed: %s", err.Error()),
	}
	outputHandler := NewOutputHandler(c.GetCommonFlags())
	outputHandler.WriteOutput(output)
}

// GetCobraCommand returns the cobra command for multi-agent operations
func (c *MultiAgentCommand) GetCobraCommand() *cobra.Command {
	cmd := c.BaseCommand.GetCobraCommand()

	// Override run function
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.Execute(cmd.Context(), args)
	}

	// Add multi-agent specific flags
	cmd.Flags().StringVarP(&c.TaskType, "type", "t", "", "Task type (edit, generate, refactor, document, test, review, optimize, analyze)")
	cmd.Flags().BoolVar(&c.EnableReview, "review", true, "Enable multi-agent review process")
	cmd.Flags().IntVar(&c.MaxAgents, "max-agents", 5, "Maximum number of agents to use")
	cmd.Flags().StringSliceVar(&c.Reviewers, "reviewers", []string{}, "Specific reviewer specializations (security, performance, architecture, testing)")
	cmd.Flags().BoolVar(&c.Secure, "secure", false, "Add security-focused reviewer")
	cmd.Flags().BoolVar(&c.Fast, "fast", false, "Add performance-focused reviewer")
	cmd.Flags().BoolVar(&c.Maintainable, "maintainable", false, "Add architecture/maintainability reviewer")

	// Mark required flags
	cmd.MarkFlagRequired("type")

	// Add examples
	cmd.Example = `  # Edit files with security review
  sigil multi edit "Add input validation" --file auth.go --secure

  # Generate tests with multiple reviewers
  sigil multi generate "Create unit tests" --dir src/ --reviewers security,performance

  # Refactor with all quality aspects
  sigil multi refactor "Optimize database access" --file db.go --secure --fast --maintainable

  # Analyze code quality
  sigil multi analyze "Review error handling" --git --staged --review`

	return cmd
}

// Create the global multi-agent command instance
var multiAgentCmd = NewMultiAgentCommand().GetCobraCommand()
