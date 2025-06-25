// Package cli provides command-line interface implementations
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dshills/sigil/internal/agent"
	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/git"
	"github.com/dshills/sigil/internal/logger"
)

// ReviewCommand handles code review operations
type ReviewCommand struct {
	*BaseCommand
	Files            []string
	Focus            []string
	Severity         string
	Format           string
	OutputFile       string
	IncludeTests     bool
	CheckSecurity    bool
	CheckPerformance bool
	CheckStyle       bool
	AutoFix          bool
	startTime        time.Time
}

// NewReviewCommand creates a new review command
func NewReviewCommand() *ReviewCommand {
	return &ReviewCommand{
		BaseCommand: NewBaseCommand("review", "Review code with AI-powered analysis",
			"Perform comprehensive code review using AI-powered analysis and best practices."),
		Severity:  "warning",
		Format:    "markdown",
		startTime: time.Now(),
	}
}

// Execute runs the review command
func (c *ReviewCommand) Execute(ctx context.Context) error {
	logger.Info("starting code review", "files", c.Files, "focus", c.Focus, "severity", c.Severity)

	// Validate Git repository
	gitRepo, err := git.NewRepository(".")
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "Execute", "failed to open git repository")
	}

	// Validate inputs
	if err := c.validateInputs(); err != nil {
		return err
	}

	// Create task for agent processing
	task, err := c.createReviewTask()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to create review task")
	}

	// Execute review
	result, err := c.executeReview(ctx, task)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to execute review")
	}

	// Process and output result
	if err := c.outputResult(result); err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to output result")
	}

	// Auto-fix if requested
	if c.AutoFix && result.Status == agent.StatusSuccess {
		if err := c.applyAutoFixes(result, gitRepo); err != nil {
			logger.Warn("failed to apply auto-fixes", "error", err)
		}
	}

	return nil
}

// validateInputs validates the command inputs
func (c *ReviewCommand) validateInputs() error {
	if len(c.Files) == 0 {
		return errors.New(errors.ErrorTypeInput, "validateInputs", "no files specified for review")
	}

	for _, file := range c.Files {
		if !c.fileExists(file) {
			return errors.New(errors.ErrorTypeInput, "validateInputs",
				fmt.Sprintf("file does not exist: %s", file))
		}
	}

	validSeverities := []string{"error", "warning", "info", "all"}
	severityValid := false
	for _, severity := range validSeverities {
		if c.Severity == severity {
			severityValid = true
			break
		}
	}
	if !severityValid {
		return errors.New(errors.ErrorTypeInput, "validateInputs",
			fmt.Sprintf("invalid severity: %s (valid: %s)", c.Severity, strings.Join(validSeverities, ", ")))
	}

	validFormats := []string{"markdown", "text", "json", "xml", "sarif"}
	formatValid := false
	for _, format := range validFormats {
		if c.Format == format {
			formatValid = true
			break
		}
	}
	if !formatValid {
		return errors.New(errors.ErrorTypeInput, "validateInputs",
			fmt.Sprintf("invalid format: %s (valid: %s)", c.Format, strings.Join(validFormats, ", ")))
	}

	return nil
}

// createReviewTask creates a task for code review
func (c *ReviewCommand) createReviewTask() (*agent.Task, error) {
	// Read file contents
	fileContexts := make([]agent.FileContext, 0, len(c.Files))
	for _, filePath := range c.Files {
		content, err := c.readFile(filePath)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrorTypeInput, "createReviewTask",
				fmt.Sprintf("failed to read file: %s", filePath))
		}

		fileContext := agent.FileContext{
			Path:        filePath,
			Content:     content,
			Language:    c.detectLanguage(filePath),
			Purpose:     "Code to review",
			IsTarget:    true,
			IsReference: false,
		}
		fileContexts = append(fileContexts, fileContext)
	}

	// Create requirements based on flags
	requirements := []string{
		"Perform comprehensive code review",
		"Identify potential bugs, security issues, and performance problems",
		"Check code quality, maintainability, and best practices",
		"Provide constructive feedback and improvement suggestions",
	}

	if len(c.Focus) > 0 {
		requirements = append(requirements, fmt.Sprintf("Focus specifically on: %s", strings.Join(c.Focus, ", ")))
	}

	if c.IncludeTests {
		requirements = append(requirements, "Review test coverage and test quality")
	}

	requirements = append(requirements, fmt.Sprintf("Report only issues of severity %s and above", c.Severity))
	requirements = append(requirements, fmt.Sprintf("Format the review as %s", c.Format))

	// Create constraints based on flags
	var constraints []agent.Constraint
	if c.CheckSecurity {
		constraints = append(constraints, agent.Constraint{
			Type:        agent.ConstraintTypeSecurity,
			Description: "Identify security vulnerabilities and unsafe practices",
			Severity:    agent.SeverityError,
		})
	}
	if c.CheckPerformance {
		constraints = append(constraints, agent.Constraint{
			Type:        agent.ConstraintTypePerformance,
			Description: "Identify performance bottlenecks and optimization opportunities",
			Severity:    agent.SeverityWarning,
		})
	}
	if c.CheckStyle {
		constraints = append(constraints, agent.Constraint{
			Type:        agent.ConstraintTypeStyle,
			Description: "Check code style and formatting consistency",
			Severity:    agent.SeverityInfo,
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
		ID:          fmt.Sprintf("review_%d", c.startTime.Unix()),
		Type:        agent.TaskTypeReview,
		Description: c.buildDescription(),
		Context: agent.TaskContext{
			Files:        fileContexts,
			Requirements: requirements,
			ProjectInfo:  projectInfo,
		},
		Constraints: constraints,
		Priority:    agent.PriorityHigh,
		CreatedAt:   c.startTime,
	}

	return task, nil
}

// buildDescription builds the task description
func (c *ReviewCommand) buildDescription() string {
	description := "Perform comprehensive code review of the provided files"

	if len(c.Focus) > 0 {
		description += fmt.Sprintf(" with focus on: %s", strings.Join(c.Focus, ", "))
	}

	if c.AutoFix {
		description += " (auto-fix enabled)"
	}

	return description
}

// executeReview executes the review using the agent system
func (c *ReviewCommand) executeReview(ctx context.Context, task *agent.Task) (*agent.OrchestrationResult, error) {
	logger.Info("executing code review with agent system")

	// Create agent factory and orchestrator
	factory := agent.NewFactory(nil, agent.DefaultOrchestrationConfig()) // No sandbox needed for review
	orchestrator, err := factory.CreateOrchestrator()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "executeReview", "failed to create orchestrator")
	}

	// Execute task
	result, err := orchestrator.ExecuteTask(ctx, *task)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "executeReview", "task execution failed")
	}

	if result.Status != agent.StatusSuccess {
		return nil, errors.New(errors.ErrorTypeInternal, "executeReview",
			fmt.Sprintf("review failed with status: %s", result.Status))
	}

	logger.Info("code review completed", "status", result.Status, "findings", len(result.Results))
	return result, nil
}

// outputResult outputs the review result
func (c *ReviewCommand) outputResult(result *agent.OrchestrationResult) error {
	if result.FinalResult == nil {
		return errors.New(errors.ErrorTypeInternal, "outputResult", "no final result available")
	}

	review := result.FinalResult.Reasoning
	if review == "" && len(result.FinalResult.Artifacts) > 0 {
		// Use artifact content if reasoning is empty
		review = result.FinalResult.Artifacts[0].Content
	}

	if review == "" {
		return errors.New(errors.ErrorTypeInternal, "outputResult", "no review content generated")
	}

	// Format the output
	formatted, err := c.formatOutput(review, result)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "outputResult", "failed to format output")
	}

	// Write to file or stdout
	if c.OutputFile != "" {
		if err := c.writeFile(c.OutputFile, formatted); err != nil {
			return errors.Wrap(err, errors.ErrorTypeInternal, "outputResult",
				fmt.Sprintf("failed to write output file: %s", c.OutputFile))
		}
		fmt.Printf("Review written to: %s\n", c.OutputFile)
	} else {
		fmt.Print(formatted)
	}

	return nil
}

// formatOutput formats the review based on the requested format
func (c *ReviewCommand) formatOutput(content string, result *agent.OrchestrationResult) (string, error) {
	switch c.Format {
	case "markdown":
		return c.formatMarkdown(content, result), nil
	case "text":
		return c.formatText(content, result), nil
	case "json":
		return c.formatJSON(content, result), nil
	case "xml":
		return c.formatXML(content, result), nil
	case "sarif":
		return c.formatSARIF(content, result), nil
	default:
		return content, nil
	}
}

// formatMarkdown formats content as markdown
func (c *ReviewCommand) formatMarkdown(content string, result *agent.OrchestrationResult) string {
	var output strings.Builder

	output.WriteString("# Code Review Report\n\n")

	if len(c.Focus) > 0 {
		output.WriteString(fmt.Sprintf("**Focus Areas:** %s\n\n", strings.Join(c.Focus, ", ")))
	}

	output.WriteString("**Files Reviewed:**\n")
	for _, file := range c.Files {
		output.WriteString(fmt.Sprintf("- `%s`\n", file))
	}
	output.WriteString("\n")

	output.WriteString(fmt.Sprintf("**Severity Filter:** %s\n", c.Severity))
	output.WriteString(fmt.Sprintf("**Review Status:** %s\n", result.Status))
	output.WriteString(fmt.Sprintf("**Total Findings:** %d\n\n", len(result.Results)))

	output.WriteString("## Review Details\n\n")
	output.WriteString(content)
	output.WriteString("\n")

	return output.String()
}

// formatText formats content as plain text
func (c *ReviewCommand) formatText(content string, result *agent.OrchestrationResult) string {
	var output strings.Builder

	output.WriteString("CODE REVIEW REPORT\n")
	output.WriteString("==================\n\n")

	if len(c.Focus) > 0 {
		output.WriteString(fmt.Sprintf("Focus Areas: %s\n\n", strings.Join(c.Focus, ", ")))
	}

	output.WriteString("Files Reviewed:\n")
	for _, file := range c.Files {
		output.WriteString(fmt.Sprintf("  - %s\n", file))
	}
	output.WriteString("\n")

	output.WriteString(fmt.Sprintf("Severity Filter: %s\n", c.Severity))
	output.WriteString(fmt.Sprintf("Review Status: %s\n", result.Status))
	output.WriteString(fmt.Sprintf("Total Findings: %d\n\n", len(result.Results)))

	output.WriteString("Review Details:\n")
	output.WriteString("---------------\n")
	output.WriteString(content)
	output.WriteString("\n")

	return output.String()
}

// formatJSON formats content as JSON
func (c *ReviewCommand) formatJSON(content string, result *agent.OrchestrationResult) string {
	data := map[string]interface{}{
		"review": map[string]interface{}{
			"focus_areas":    c.Focus,
			"files":          c.Files,
			"severity":       c.Severity,
			"status":         string(result.Status),
			"findings_count": len(result.Results),
			"timestamp":      c.startTime.Format("2006-01-02T15:04:05Z07:00"),
			"content":        content,
		},
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		// Fallback to simple formatting if JSON marshaling fails
		return fmt.Sprintf(`{"error": "Failed to format JSON: %s"}`, err.Error())
	}
	return string(jsonBytes)
}

// formatXML formats content as XML
func (c *ReviewCommand) formatXML(content string, result *agent.OrchestrationResult) string {
	var output strings.Builder

	output.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	output.WriteString("<review>\n")
	output.WriteString(fmt.Sprintf("  <timestamp>%s</timestamp>\n", c.startTime.Format("2006-01-02T15:04:05Z07:00")))
	output.WriteString(fmt.Sprintf("  <severity>%s</severity>\n", c.Severity))
	output.WriteString(fmt.Sprintf("  <status>%s</status>\n", result.Status))
	output.WriteString(fmt.Sprintf("  <findings_count>%d</findings_count>\n", len(result.Results)))

	output.WriteString("  <focus_areas>\n")
	for _, focus := range c.Focus {
		output.WriteString(fmt.Sprintf("    <area>%s</area>\n", focus))
	}
	output.WriteString("  </focus_areas>\n")

	output.WriteString("  <files>\n")
	for _, file := range c.Files {
		output.WriteString(fmt.Sprintf("    <file>%s</file>\n", file))
	}
	output.WriteString("  </files>\n")

	output.WriteString("  <content><![CDATA[\n")
	output.WriteString(content)
	output.WriteString("\n  ]]></content>\n")
	output.WriteString("</review>\n")

	return output.String()
}

// formatSARIF formats content as SARIF (Static Analysis Results Interchange Format)
func (c *ReviewCommand) formatSARIF(content string, result *agent.OrchestrationResult) string {
	return fmt.Sprintf(`{
  "$schema": "https://schemastore.azurewebsites.net/schemas/json/sarif-2.1.0.json",
  "version": "2.1.0",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "Sigil Code Review",
          "version": "1.0.0",
          "informationUri": "https://github.com/dshills/sigil"
        }
      },
      "results": [
        {
          "ruleId": "comprehensive-review",
          "level": "%s",
          "message": {
            "text": "Code review completed with %d findings"
          },
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": {
                  "uri": "%s"
                }
              }
            }
          ]
        }
      ]
    }
  ]
}`, c.Severity, len(result.Results), c.Files[0])
}

// applyAutoFixes applies automatic fixes from the review result
func (c *ReviewCommand) applyAutoFixes(result *agent.OrchestrationResult, gitRepo *git.Repository) error {
	if result.FinalResult == nil || len(result.FinalResult.Proposals) == 0 {
		logger.Info("no auto-fixes available")
		return nil
	}

	logger.Info("applying auto-fixes", "proposals", len(result.FinalResult.Proposals))

	for _, proposal := range result.FinalResult.Proposals {
		if err := c.applyProposal(proposal); err != nil {
			logger.Warn("failed to apply proposal", "proposal_id", proposal.ID, "error", err)
			continue
		}
	}

	// Commit changes if any were made
	if err := gitRepo.Add("."); err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "applyAutoFixes", "failed to stage changes")
	}

	message := fmt.Sprintf("sigil review: auto-fix applied (%d fixes)", len(result.FinalResult.Proposals))
	if err := gitRepo.Commit(message); err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "applyAutoFixes", "failed to commit auto-fixes")
	}

	logger.Info("auto-fixes committed", "message", message)
	return nil
}

// applyProposal applies a single proposal from the review
func (c *ReviewCommand) applyProposal(proposal agent.Proposal) error {
	for _, change := range proposal.Changes {
		switch change.Type {
		case agent.ChangeTypeUpdate:
			if err := c.writeFile(change.Path, change.NewContent); err != nil {
				return errors.Wrap(err, errors.ErrorTypeInternal, "applyProposal",
					fmt.Sprintf("failed to apply change to file: %s", change.Path))
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
			logger.Debug("change type not yet implemented", "type", change.Type, "path", change.Path)
		default:
			logger.Debug("skipping unsupported change type", "type", change.Type, "path", change.Path)
		}
	}
	return nil
}

// detectProjectLanguage detects the primary language of the project
func (c *ReviewCommand) detectProjectLanguage() string {
	if c.fileExists("go.mod") || c.fileExists("main.go") {
		return "go"
	}
	if c.fileExists("package.json") {
		return "javascript"
	}
	if c.fileExists("requirements.txt") || c.fileExists("setup.py") {
		return "python"
	}
	if c.fileExists("pom.xml") || c.fileExists("build.gradle") {
		return "java"
	}
	return "text"
}

// detectFramework detects the framework being used
func (c *ReviewCommand) detectFramework() string {
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
func (c *ReviewCommand) detectLanguage(filePath string) string {
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

// CreateCobraCommand creates the cobra command for review
func (c *ReviewCommand) CreateCobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review [files...]",
		Short: "Review code with AI-powered analysis",
		Long: `Perform comprehensive code review using AI-powered analysis and best practices.

The review command analyzes code for bugs, security issues, performance problems,
and adherence to best practices. It can focus on specific areas and output results
in various formats.

Examples:
  sigil review main.go
  sigil review src/ --focus security,performance
  sigil review *.go --severity error --format json --output review.json
  sigil review project/ --auto-fix --check-security`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Files = args
			ctx := cmd.Context()
			return c.Execute(ctx)
		},
	}

	// Add flags
	cmd.Flags().StringSliceVar(&c.Focus, "focus", []string{}, "Focus areas (security,performance,style,testing)")
	cmd.Flags().StringVar(&c.Severity, "severity", "warning", "Minimum severity to report (error,warning,info,all)")
	cmd.Flags().StringVar(&c.Format, "format", "markdown", "Output format (markdown,text,json,xml,sarif)")
	cmd.Flags().StringVarP(&c.OutputFile, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().BoolVar(&c.IncludeTests, "include-tests", false, "Include test coverage analysis")
	cmd.Flags().BoolVar(&c.CheckSecurity, "check-security", false, "Focus on security issues")
	cmd.Flags().BoolVar(&c.CheckPerformance, "check-performance", false, "Focus on performance issues")
	cmd.Flags().BoolVar(&c.CheckStyle, "check-style", false, "Focus on style and formatting")
	cmd.Flags().BoolVar(&c.AutoFix, "auto-fix", false, "Automatically apply fixes where possible")

	return cmd
}

// fileExists checks if a file exists
func (c *ReviewCommand) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readFile reads a file's content
func (c *ReviewCommand) readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// writeFile writes content to a file
func (c *ReviewCommand) writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

// deleteFile deletes a file
func (c *ReviewCommand) deleteFile(path string) error {
	return os.Remove(path)
}

// Legacy review command for backwards compatibility
var reviewCmd = NewReviewCommand().CreateCobraCommand()

// init registers the command during package initialization
func init() {
	// This ensures the command is available when root.go calls it
	reviewCmd = NewReviewCommand().CreateCobraCommand()
}
