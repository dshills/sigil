// Package cli provides command-line interface implementations
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dshills/sigil/internal/agent"
	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/git"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/sandbox"
)

// EditCommand handles code editing operations
type EditCommand struct {
	*BaseCommand
	Files       []string
	Description string
	Secure      bool
	Fast        bool
	Maintain    bool
	AutoCommit  bool
	Branch      string
	UseAgent    bool
	startTime   time.Time
}

// NewEditCommand creates a new edit command
func NewEditCommand() *EditCommand {
	return &EditCommand{
		BaseCommand: NewBaseCommand("edit", "Edit files with intelligent code transformations",
			"Edit specified files using AI-powered code transformations."),
		startTime: time.Now(),
	}
}

// Execute runs the edit command
func (c *EditCommand) Execute(ctx context.Context) error {
	logger.Info("starting edit operation", "files", c.Files, "description", c.Description)

	// Validate Git repository
	gitRepo, err := git.NewRepository(".")
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "Execute", "failed to open git repository")
	}

	// Validate files exist
	if err := c.validateFiles(); err != nil {
		return err
	}

	// Create task for agent processing
	task, err := c.createEditTask()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to create edit task")
	}

	// Process with agent if enabled
	if c.UseAgent {
		return c.executeWithAgent(ctx, task, gitRepo)
	}

	// Execute traditional edit operation
	return c.executeDirectEdit(ctx, gitRepo)
}

// validateFiles checks that all specified files exist and are within the Git repository
func (c *EditCommand) validateFiles() error {
	if len(c.Files) == 0 {
		return errors.New(errors.ErrorTypeInput, "validateFiles", "no files specified for editing")
	}

	for _, file := range c.Files {
		if !c.fileExists(file) {
			return errors.New(errors.ErrorTypeInput, "validateFiles",
				fmt.Sprintf("file does not exist: %s", file))
		}
	}

	return nil
}

// createEditTask creates a task for agent processing
func (c *EditCommand) createEditTask() (*agent.Task, error) {
	// Read file contents
	fileContexts := make([]agent.FileContext, 0, len(c.Files))
	for _, filePath := range c.Files {
		content, err := c.readFile(filePath)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrorTypeInput, "createEditTask",
				fmt.Sprintf("failed to read file: %s", filePath))
		}

		fileContext := agent.FileContext{
			Path:        filePath,
			Content:     content,
			Language:    c.detectLanguage(filePath),
			Purpose:     "Target file for editing",
			IsTarget:    true,
			IsReference: false,
		}
		fileContexts = append(fileContexts, fileContext)
	}

	// Create constraints based on flags
	var constraints []agent.Constraint
	if c.Secure {
		constraints = append(constraints, agent.Constraint{
			Type:        agent.ConstraintTypeSecurity,
			Description: "Ensure secure coding practices and avoid vulnerabilities",
			Severity:    agent.SeverityError,
		})
	}
	if c.Fast {
		constraints = append(constraints, agent.Constraint{
			Type:        agent.ConstraintTypePerformance,
			Description: "Optimize for performance and efficiency",
			Severity:    agent.SeverityWarning,
		})
	}
	if c.Maintain {
		constraints = append(constraints, agent.Constraint{
			Type:        agent.ConstraintTypeStyle,
			Description: "Follow best practices for maintainable code",
			Severity:    agent.SeverityWarning,
		})
	}

	// Detect project info
	projectInfo := agent.ProjectInfo{
		Language:  c.detectProjectLanguage(),
		Framework: c.detectFramework(),
		Style:     "standard",
	}

	// Create task
	task := &agent.Task{
		ID:          fmt.Sprintf("edit_%d", c.startTime.Unix()),
		Type:        agent.TaskTypeEdit,
		Description: c.Description,
		Context: agent.TaskContext{
			Files:        fileContexts,
			Requirements: []string{"Edit the specified files according to the description"},
			ProjectInfo:  projectInfo,
		},
		Constraints: constraints,
		Priority:    agent.PriorityMedium,
		CreatedAt:   c.startTime,
	}

	return task, nil
}

// executeWithAgent processes the edit using the agent system
func (c *EditCommand) executeWithAgent(ctx context.Context, task *agent.Task, gitRepo *git.Repository) error {
	logger.Info("executing edit with agent system")

	// Create sandbox
	sandbox, err := c.createSandbox(gitRepo)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "executeWithAgent", "failed to create sandbox")
	}
	defer func() {
		if err := sandbox.Cleanup(); err != nil {
			logger.Warn("failed to cleanup sandbox", "error", err)
		}
	}()

	// Create agent factory and orchestrator
	factory := agent.NewFactory(sandbox, agent.DefaultOrchestrationConfig())
	orchestrator, err := factory.CreateOrchestrator()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "executeWithAgent", "failed to create orchestrator")
	}

	// Execute task
	result, err := orchestrator.ExecuteTask(ctx, *task)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "executeWithAgent", "task execution failed")
	}

	// Process results
	if err := c.processAgentResult(result, gitRepo); err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "executeWithAgent", "failed to process agent result")
	}

	logger.Info("edit operation completed with agent", "status", result.Status, "proposals", len(result.Results))
	return nil
}

// executeDirectEdit processes the edit without agent system
func (c *EditCommand) executeDirectEdit(ctx context.Context, gitRepo *git.Repository) error {
	logger.Info("executing direct edit operation")

	// For direct edit, we would implement traditional file editing logic
	// This is a placeholder for non-agent editing capabilities

	fmt.Printf("Direct edit not yet implemented. Files to edit: %v\n", c.Files)
	fmt.Printf("Description: %s\n", c.Description)

	return nil
}

// processAgentResult processes the results from agent execution
func (c *EditCommand) processAgentResult(result *agent.OrchestrationResult, gitRepo *git.Repository) error {
	if result.Status != agent.StatusSuccess {
		return errors.New(errors.ErrorTypeInternal, "processAgentResult",
			fmt.Sprintf("agent execution failed with status: %s", result.Status))
	}

	// Process proposals from the final result
	if result.FinalResult != nil && len(result.FinalResult.Proposals) > 0 {
		for _, proposal := range result.FinalResult.Proposals {
			if err := c.applyProposal(proposal, gitRepo); err != nil {
				return errors.Wrap(err, errors.ErrorTypeInternal, "processAgentResult",
					fmt.Sprintf("failed to apply proposal: %s", proposal.ID))
			}
		}
	}

	// Auto-commit if enabled
	if c.AutoCommit {
		if err := c.commitChanges(gitRepo, result); err != nil {
			logger.Warn("failed to auto-commit changes", "error", err)
		}
	}

	return nil
}

// applyProposal applies a proposal's changes to the repository
func (c *EditCommand) applyProposal(proposal agent.Proposal, gitRepo *git.Repository) error {
	logger.Debug("applying proposal", "proposal_id", proposal.ID, "changes", len(proposal.Changes))

	for _, change := range proposal.Changes {
		switch change.Type {
		case agent.ChangeTypeUpdate:
			if err := c.writeFile(change.Path, change.NewContent); err != nil {
				return errors.Wrap(err, errors.ErrorTypeInternal, "applyProposal",
					fmt.Sprintf("failed to write file: %s", change.Path))
			}
		case agent.ChangeTypeCreate:
			if err := c.writeFile(change.Path, change.NewContent); err != nil {
				return errors.Wrap(err, errors.ErrorTypeInternal, "applyProposal",
					fmt.Sprintf("failed to create file: %s", change.Path))
			}
		case agent.ChangeTypeDelete:
			if err := c.deleteFile(change.Path); err != nil {
				return errors.Wrap(err, errors.ErrorTypeInternal, "applyProposal",
					fmt.Sprintf("failed to delete file: %s", change.Path))
			}
		case agent.ChangeTypeMove, agent.ChangeTypeRename:
			logger.Warn("change type not yet implemented", "type", change.Type, "path", change.Path)
		default:
			logger.Warn("unsupported change type", "type", change.Type, "path", change.Path)
		}
	}

	return nil
}

// commitChanges commits the changes to Git if auto-commit is enabled
func (c *EditCommand) commitChanges(gitRepo *git.Repository, result *agent.OrchestrationResult) error {
	message := fmt.Sprintf("sigil edit: %s", c.Description)
	if len(message) > 50 {
		message = message[:47] + "..."
	}

	if err := gitRepo.Add("."); err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "commitChanges", "failed to stage changes")
	}

	if err := gitRepo.Commit(message); err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "commitChanges", "failed to commit changes")
	}

	logger.Info("auto-committed changes", "message", message)
	return nil
}

// detectProjectLanguage detects the primary language of the project
func (c *EditCommand) detectProjectLanguage() string {
	// Check for Go files
	if c.fileExists("go.mod") || c.fileExists("main.go") {
		return "go"
	}

	// Check for JavaScript/TypeScript
	if c.fileExists("package.json") {
		return "javascript"
	}

	// Check for Python
	if c.fileExists("requirements.txt") || c.fileExists("setup.py") || c.fileExists("pyproject.toml") {
		return "python"
	}

	// Check for Java
	if c.fileExists("pom.xml") || c.fileExists("build.gradle") {
		return "java"
	}

	return "text"
}

// detectFramework detects the framework being used
func (c *EditCommand) detectFramework() string {
	// Check for specific framework files
	if c.fileExists("next.config.js") {
		return "next.js"
	}
	if c.fileExists("angular.json") {
		return "angular"
	}
	if c.fileExists("vue.config.js") {
		return "vue"
	}

	return ""
}

// detectLanguage detects the language of a specific file
func (c *EditCommand) detectLanguage(filePath string) string {
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
	} else if strings.HasSuffix(filePath, ".rs") {
		return "rust"
	}
	return "text"
}

// CreateCobraCommand creates the cobra command for edit
func (c *EditCommand) CreateCobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit [files...]",
		Short: "Edit files with intelligent code transformations",
		Long: `Edit specified files using AI-powered code transformations.

The edit command can modify existing files based on natural language descriptions.
It supports both direct editing and agent-based collaborative editing with review.

Examples:
  sigil edit main.go --description "Add error handling to the main function"
  sigil edit *.go --description "Refactor to use interfaces" --secure
  sigil edit src/ --description "Add logging" --agent --auto-commit`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Files = args
			ctx := cmd.Context()
			return c.Execute(ctx)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&c.Description, "description", "d", "", "Description of the edit to perform")
	cmd.Flags().BoolVar(&c.Secure, "secure", false, "Apply security-focused constraints")
	cmd.Flags().BoolVar(&c.Fast, "fast", false, "Apply performance-focused constraints")
	cmd.Flags().BoolVar(&c.Maintain, "maintain", false, "Apply maintainability-focused constraints")
	cmd.Flags().BoolVar(&c.AutoCommit, "auto-commit", false, "Automatically commit changes")
	cmd.Flags().StringVar(&c.Branch, "branch", "", "Create and switch to a new branch")
	cmd.Flags().BoolVar(&c.UseAgent, "agent", true, "Use agent system for editing")

	// Mark required flags
	if err := cmd.MarkFlagRequired("description"); err != nil {
		// Log error but continue - this is a setup error that shouldn't prevent command creation
		logger.Warn("failed to mark flag as required", "flag", "description", "error", err)
	}

	return cmd
}

// fileExists checks if a file exists
func (c *EditCommand) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readFile reads a file's content
func (c *EditCommand) readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// writeFile writes content to a file
func (c *EditCommand) writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

// deleteFile deletes a file
func (c *EditCommand) deleteFile(path string) error {
	return os.Remove(path)
}

// createSandbox creates a sandbox for testing changes
func (c *EditCommand) createSandbox(gitRepo *git.Repository) (sandbox.Manager, error) {
	// Create sandbox manager
	return sandbox.NewManager(gitRepo)
}

// Legacy edit command implementation for backwards compatibility
var editCmd = NewEditCommand().CreateCobraCommand()

// init registers the command during package initialization
func init() {
	// This ensures the command is available when root.go calls it
	editCmd = NewEditCommand().CreateCobraCommand()
}
