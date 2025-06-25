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
	"github.com/dshills/sigil/internal/logger"
)

// ExplainCommand handles code explanation operations
type ExplainCommand struct {
	*BaseCommand
	Files       []string
	Query       string
	Detailed    bool
	Format      string
	OutputFile  string
	Interactive bool
	startTime   time.Time
}

// NewExplainCommand creates a new explain command
func NewExplainCommand() *ExplainCommand {
	return &ExplainCommand{
		BaseCommand: NewBaseCommand("explain", "Explain code files with AI-powered analysis",
			"Explain and analyze code files using AI-powered natural language explanations."),
		Format:    "markdown",
		startTime: time.Now(),
	}
}

// Execute runs the explain command
func (c *ExplainCommand) Execute(ctx context.Context) error {
	logger.Info("starting explain operation", "files", c.Files, "query", c.Query)

	// Validate inputs
	if err := c.validateInputs(); err != nil {
		return err
	}

	// Create task for agent processing
	task, err := c.createExplainTask()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to create explain task")
	}

	// Execute explanation
	result, err := c.executeExplanation(ctx, task)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to execute explanation")
	}

	// Output result
	return c.outputResult(result)
}

// validateInputs validates the command inputs
func (c *ExplainCommand) validateInputs() error {
	if len(c.Files) == 0 {
		return errors.New(errors.ErrorTypeInput, "validateInputs", "no files specified for explanation")
	}

	for _, file := range c.Files {
		if !c.fileExists(file) {
			return errors.New(errors.ErrorTypeInput, "validateInputs",
				fmt.Sprintf("file does not exist: %s", file))
		}
	}

	validFormats := []string{"markdown", "text", "json", "html"}
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

// createExplainTask creates a task for explanation
func (c *ExplainCommand) createExplainTask() (*agent.Task, error) {
	// Read file contents
	fileContexts := make([]agent.FileContext, 0, len(c.Files))
	for _, filePath := range c.Files {
		content, err := c.readFile(filePath)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrorTypeInput, "createExplainTask",
				fmt.Sprintf("failed to read file: %s", filePath))
		}

		fileContext := agent.FileContext{
			Path:        filePath,
			Content:     content,
			Language:    c.detectLanguage(filePath),
			Purpose:     "Code to explain and analyze",
			IsTarget:    false,
			IsReference: true,
		}
		fileContexts = append(fileContexts, fileContext)
	}

	// Create requirements based on query and flags
	requirements := []string{
		"Provide a clear and comprehensive explanation of the code",
		"Explain the purpose, functionality, and key concepts",
		"Identify important patterns, algorithms, and design decisions",
	}

	if c.Query != "" {
		requirements = append(requirements, fmt.Sprintf("Focus on: %s", c.Query))
	}

	if c.Detailed {
		requirements = append(requirements,
			"Provide detailed explanations including line-by-line analysis where relevant",
			"Explain complex algorithms and data structures in depth",
			"Include examples and use cases")
	}

	requirements = append(requirements, fmt.Sprintf("Format the explanation as %s", c.Format))

	// Detect project info
	projectInfo := agent.ProjectInfo{
		Language:  c.detectProjectLanguage(),
		Framework: c.detectFramework(),
		Style:     "standard",
	}

	// Create task
	task := &agent.Task{
		ID:          fmt.Sprintf("explain_%d", c.startTime.Unix()),
		Type:        agent.TaskTypeAnalyze,
		Description: c.buildDescription(),
		Context: agent.TaskContext{
			Files:        fileContexts,
			Requirements: requirements,
			ProjectInfo:  projectInfo,
		},
		Priority:  agent.PriorityMedium,
		CreatedAt: c.startTime,
	}

	return task, nil
}

// buildDescription builds the task description
func (c *ExplainCommand) buildDescription() string {
	description := "Explain and analyze the provided code files"

	if c.Query != "" {
		description += fmt.Sprintf(" with focus on: %s", c.Query)
	}

	if c.Detailed {
		description += " (detailed analysis requested)"
	}

	return description
}

// executeExplanation executes the explanation using the agent system
func (c *ExplainCommand) executeExplanation(ctx context.Context, task *agent.Task) (*agent.OrchestrationResult, error) {
	logger.Info("executing explanation with agent system")

	// Create agent factory and orchestrator
	factory := agent.NewFactory(nil, agent.DefaultOrchestrationConfig()) // No sandbox needed for explanation
	orchestrator, err := factory.CreateOrchestrator()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "executeExplanation", "failed to create orchestrator")
	}

	// Execute task
	result, err := orchestrator.ExecuteTask(ctx, *task)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "executeExplanation", "task execution failed")
	}

	if result.Status != agent.StatusSuccess {
		return nil, errors.New(errors.ErrorTypeInternal, "executeExplanation",
			fmt.Sprintf("explanation failed with status: %s", result.Status))
	}

	logger.Info("explanation completed", "status", result.Status)
	return result, nil
}

// outputResult outputs the explanation result
func (c *ExplainCommand) outputResult(result *agent.OrchestrationResult) error {
	if result.FinalResult == nil {
		return errors.New(errors.ErrorTypeInternal, "outputResult", "no final result available")
	}

	explanation := result.FinalResult.Reasoning
	if explanation == "" && len(result.FinalResult.Artifacts) > 0 {
		// Use artifact content if reasoning is empty
		explanation = result.FinalResult.Artifacts[0].Content
	}

	if explanation == "" {
		return errors.New(errors.ErrorTypeInternal, "outputResult", "no explanation content generated")
	}

	// Format the output
	formatted, err := c.formatOutput(explanation)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "outputResult", "failed to format output")
	}

	// Write to file or stdout
	if c.OutputFile != "" {
		if err := c.writeFile(c.OutputFile, formatted); err != nil {
			return errors.Wrap(err, errors.ErrorTypeInternal, "outputResult",
				fmt.Sprintf("failed to write output file: %s", c.OutputFile))
		}
		fmt.Printf("Explanation written to: %s\n", c.OutputFile)
	} else {
		fmt.Print(formatted)
	}

	return nil
}

// formatOutput formats the explanation based on the requested format
func (c *ExplainCommand) formatOutput(content string) (string, error) {
	switch c.Format {
	case FormatMarkdown:
		return c.formatMarkdown(content), nil
	case "text":
		return c.formatText(content), nil
	case "json":
		return c.formatJSON(content), nil
	case FormatHTML:
		return c.formatHTML(content), nil
	default:
		return content, nil
	}
}

// formatMarkdown formats content as markdown
func (c *ExplainCommand) formatMarkdown(content string) string {
	var result strings.Builder

	result.WriteString("# Code Explanation\n\n")

	if c.Query != "" {
		result.WriteString(fmt.Sprintf("**Query:** %s\n\n", c.Query))
	}

	result.WriteString("**Files analyzed:**\n")
	for _, file := range c.Files {
		result.WriteString(fmt.Sprintf("- `%s`\n", file))
	}
	result.WriteString("\n")

	result.WriteString("## Explanation\n\n")
	result.WriteString(content)
	result.WriteString("\n")

	return result.String()
}

// formatText formats content as plain text
func (c *ExplainCommand) formatText(content string) string {
	var result strings.Builder

	result.WriteString("CODE EXPLANATION\n")
	result.WriteString("================\n\n")

	if c.Query != "" {
		result.WriteString(fmt.Sprintf("Query: %s\n\n", c.Query))
	}

	result.WriteString("Files analyzed:\n")
	for _, file := range c.Files {
		result.WriteString(fmt.Sprintf("  - %s\n", file))
	}
	result.WriteString("\n")

	result.WriteString("Explanation:\n")
	result.WriteString("------------\n")
	result.WriteString(content)
	result.WriteString("\n")

	return result.String()
}

// formatJSON formats content as JSON
func (c *ExplainCommand) formatJSON(content string) string {
	data := map[string]interface{}{
		"query":       c.Query,
		"files":       c.Files,
		"explanation": content,
		"format":      "json",
		"timestamp":   c.startTime.Format("2006-01-02T15:04:05Z07:00"),
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		// Fallback to simple formatting if JSON marshaling fails
		return fmt.Sprintf(`{"error": "Failed to format JSON: %s"}`, err.Error())
	}
	return string(jsonBytes)
}

// formatHTML formats content as HTML
func (c *ExplainCommand) formatHTML(content string) string {
	var result strings.Builder

	result.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	result.WriteString("<title>Code Explanation</title>\n")
	result.WriteString("<style>body{font-family:Arial,sans-serif;margin:40px;}</style>\n")
	result.WriteString("</head>\n<body>\n")

	result.WriteString("<h1>Code Explanation</h1>\n")

	if c.Query != "" {
		result.WriteString(fmt.Sprintf("<p><strong>Query:</strong> %s</p>\n", c.Query))
	}

	result.WriteString("<h2>Files Analyzed</h2>\n<ul>\n")
	for _, file := range c.Files {
		result.WriteString(fmt.Sprintf("<li><code>%s</code></li>\n", file))
	}
	result.WriteString("</ul>\n")

	result.WriteString("<h2>Explanation</h2>\n")
	result.WriteString("<div>\n")
	// Simple conversion - in real implementation, you'd properly escape HTML
	result.WriteString(strings.ReplaceAll(content, "\n", "<br>\n"))
	result.WriteString("\n</div>\n")

	result.WriteString("</body>\n</html>\n")

	return result.String()
}

// detectProjectLanguage detects the primary language of the project
func (c *ExplainCommand) detectProjectLanguage() string {
	if c.fileExists("go.mod") || c.fileExists("main.go") {
		return "go"
	}
	if c.fileExists("package.json") {
		return LangJavaScript
	}
	if c.fileExists("requirements.txt") || c.fileExists("setup.py") {
		return LangPython
	}
	if c.fileExists("pom.xml") || c.fileExists("build.gradle") {
		return LangJava
	}
	return "text"
}

// detectFramework detects the framework being used
func (c *ExplainCommand) detectFramework() string {
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
func (c *ExplainCommand) detectLanguage(filePath string) string {
	if strings.HasSuffix(filePath, ".go") {
		return "go"
	} else if strings.HasSuffix(filePath, ".js") || strings.HasSuffix(filePath, ".ts") {
		return LangJavaScript
	} else if strings.HasSuffix(filePath, ".py") {
		return LangPython
	} else if strings.HasSuffix(filePath, ".java") {
		return LangJava
	} else if strings.HasSuffix(filePath, ".cpp") || strings.HasSuffix(filePath, ".c") {
		return LangCPP
	} else if strings.HasSuffix(filePath, ".rs") {
		return LangRust
	}
	return string(InputTypeText)
}

// CreateCobraCommand creates the cobra command for explain
func (c *ExplainCommand) CreateCobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain [files...]",
		Short: "Explain code files with AI-powered analysis",
		Long: `Explain and analyze code files using AI-powered natural language explanations.

The explain command provides comprehensive explanations of code functionality,
architecture, patterns, and design decisions. It can focus on specific aspects
based on queries and provide output in multiple formats.

Examples:
  sigil explain main.go
  sigil explain src/ --query "error handling patterns"
  sigil explain *.go --detailed --format html --output explanation.html
  sigil explain service.py --query "what design patterns are used?"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Files = args
			ctx := cmd.Context()
			return c.Execute(ctx)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&c.Query, "query", "q", "", "Specific question or focus area for explanation")
	cmd.Flags().BoolVar(&c.Detailed, "detailed", false, "Provide detailed line-by-line analysis")
	cmd.Flags().StringVar(&c.Format, "format", "markdown", "Output format (markdown, text, json, html)")
	cmd.Flags().StringVarP(&c.OutputFile, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().BoolVar(&c.Interactive, "interactive", false, "Interactive explanation mode")

	return cmd
}

// fileExists checks if a file exists
func (c *ExplainCommand) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readFile reads a file's content
func (c *ExplainCommand) readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// writeFile writes content to a file
func (c *ExplainCommand) writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

// Legacy explain command for backwards compatibility
var explainCmd = NewExplainCommand().CreateCobraCommand()

// init registers the command during package initialization
func init() {
	// This ensures the command is available when root.go calls it
	explainCmd = NewExplainCommand().CreateCobraCommand()
}
