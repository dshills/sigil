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

// SummarizeCommand handles code summarization operations
type SummarizeCommand struct {
	*BaseCommand
	Files      []string
	Recursive  bool
	Brief      bool
	Focus      string
	Format     string
	OutputFile string
	startTime  time.Time
}

// NewSummarizeCommand creates a new summarize command
func NewSummarizeCommand() *SummarizeCommand {
	return &SummarizeCommand{
		BaseCommand: NewBaseCommand("summarize", "Generate code summaries with AI analysis",
			"Generate comprehensive summaries of code files and projects using AI analysis."),
		Format:    "markdown",
		startTime: time.Now(),
	}
}

// Execute runs the summarize command
func (c *SummarizeCommand) Execute(ctx context.Context) error {
	logger.Info("starting summarize operation", "files", c.Files, "focus", c.Focus)

	// Validate inputs
	if err := c.validateInputs(); err != nil {
		return err
	}

	// Create task for agent processing
	task, err := c.createSummarizeTask()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to create summarize task")
	}

	// Execute summarization
	result, err := c.executeSummarization(ctx, task)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to execute summarization")
	}

	// Output result
	return c.outputResult(result)
}

// validateInputs validates the command inputs
func (c *SummarizeCommand) validateInputs() error {
	if len(c.Files) == 0 {
		return errors.New(errors.ErrorTypeInput, "validateInputs", "no files specified for summarization")
	}

	for _, file := range c.Files {
		if !c.fileExists(file) {
			return errors.New(errors.ErrorTypeInput, "validateInputs",
				fmt.Sprintf("file does not exist: %s", file))
		}
	}

	validFormats := []string{"markdown", "text", "json", "html", "yaml"}
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

// createSummarizeTask creates a task for summarization
func (c *SummarizeCommand) createSummarizeTask() (*agent.Task, error) {
	// Read file contents
	fileContexts := make([]agent.FileContext, 0, len(c.Files))
	for _, filePath := range c.Files {
		content, err := c.readFile(filePath)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrorTypeInput, "createSummarizeTask",
				fmt.Sprintf("failed to read file: %s", filePath))
		}

		fileContext := agent.FileContext{
			Path:        filePath,
			Content:     content,
			Language:    c.detectLanguage(filePath),
			Purpose:     "Code to summarize",
			IsTarget:    false,
			IsReference: true,
		}
		fileContexts = append(fileContexts, fileContext)
	}

	// Create requirements based on flags
	requirements := []string{
		"Generate a comprehensive summary of the code",
		"Identify key components, functions, and data structures",
		"Explain the overall architecture and design patterns",
		"Highlight important dependencies and relationships",
	}

	if c.Focus != "" {
		requirements = append(requirements, fmt.Sprintf("Focus specifically on: %s", c.Focus))
	}

	if c.Brief {
		requirements = append(requirements, "Provide a concise, high-level summary")
	} else {
		requirements = append(requirements,
			"Provide detailed analysis including implementation details",
			"Include code metrics and complexity analysis where relevant")
	}

	requirements = append(requirements, fmt.Sprintf("Format the summary as %s", c.Format))

	// Detect project info
	projectInfo := agent.ProjectInfo{
		Language:  c.detectProjectLanguage(),
		Framework: c.detectFramework(),
		Style:     "standard",
	}

	// Create task
	task := &agent.Task{
		ID:          fmt.Sprintf("summarize_%d", c.startTime.Unix()),
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
func (c *SummarizeCommand) buildDescription() string {
	description := "Generate a comprehensive summary of the provided code"

	if c.Focus != "" {
		description += fmt.Sprintf(" with focus on: %s", c.Focus)
	}

	if c.Brief {
		description += " (brief summary requested)"
	}

	return description
}

// executeSummarization executes the summarization using the agent system
func (c *SummarizeCommand) executeSummarization(ctx context.Context, task *agent.Task) (*agent.OrchestrationResult, error) {
	logger.Info("executing summarization with agent system")

	// Create agent factory and orchestrator
	factory := agent.NewFactory(nil, agent.DefaultOrchestrationConfig()) // No sandbox needed for summarization
	orchestrator, err := factory.CreateOrchestrator()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "executeSummarization", "failed to create orchestrator")
	}

	// Execute task
	result, err := orchestrator.ExecuteTask(ctx, *task)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "executeSummarization", "task execution failed")
	}

	if result.Status != agent.StatusSuccess {
		return nil, errors.New(errors.ErrorTypeInternal, "executeSummarization",
			fmt.Sprintf("summarization failed with status: %s", result.Status))
	}

	logger.Info("summarization completed", "status", result.Status)
	return result, nil
}

// outputResult outputs the summarization result
func (c *SummarizeCommand) outputResult(result *agent.OrchestrationResult) error {
	if result.FinalResult == nil {
		return errors.New(errors.ErrorTypeInternal, "outputResult", "no final result available")
	}

	summary := result.FinalResult.Reasoning
	if summary == "" && len(result.FinalResult.Artifacts) > 0 {
		// Use artifact content if reasoning is empty
		summary = result.FinalResult.Artifacts[0].Content
	}

	if summary == "" {
		return errors.New(errors.ErrorTypeInternal, "outputResult", "no summary content generated")
	}

	// Format the output
	formatted, err := c.formatOutput(summary)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "outputResult", "failed to format output")
	}

	// Write to file or stdout
	if c.OutputFile != "" {
		if err := c.writeFile(c.OutputFile, formatted); err != nil {
			return errors.Wrap(err, errors.ErrorTypeInternal, "outputResult",
				fmt.Sprintf("failed to write output file: %s", c.OutputFile))
		}
		fmt.Printf("Summary written to: %s\n", c.OutputFile)
	} else {
		fmt.Print(formatted)
	}

	return nil
}

// formatOutput formats the summary based on the requested format
func (c *SummarizeCommand) formatOutput(content string) (string, error) {
	switch c.Format {
	case "markdown":
		return c.formatMarkdown(content), nil
	case "text":
		return c.formatText(content), nil
	case "json":
		return c.formatJSON(content), nil
	case "html":
		return c.formatHTML(content), nil
	case "yaml":
		return c.formatYAML(content), nil
	default:
		return content, nil
	}
}

// formatMarkdown formats content as markdown
func (c *SummarizeCommand) formatMarkdown(content string) string {
	var result strings.Builder

	result.WriteString("# Code Summary\n\n")

	if c.Focus != "" {
		result.WriteString(fmt.Sprintf("**Focus:** %s\n\n", c.Focus))
	}

	result.WriteString("**Files analyzed:**\n")
	for _, file := range c.Files {
		result.WriteString(fmt.Sprintf("- `%s`\n", file))
	}
	result.WriteString("\n")

	result.WriteString("## Summary\n\n")
	result.WriteString(content)
	result.WriteString("\n")

	return result.String()
}

// formatText formats content as plain text
func (c *SummarizeCommand) formatText(content string) string {
	var result strings.Builder

	result.WriteString("CODE SUMMARY\n")
	result.WriteString("============\n\n")

	if c.Focus != "" {
		result.WriteString(fmt.Sprintf("Focus: %s\n\n", c.Focus))
	}

	result.WriteString("Files analyzed:\n")
	for _, file := range c.Files {
		result.WriteString(fmt.Sprintf("  - %s\n", file))
	}
	result.WriteString("\n")

	result.WriteString("Summary:\n")
	result.WriteString("--------\n")
	result.WriteString(content)
	result.WriteString("\n")

	return result.String()
}

// formatJSON formats content as JSON
func (c *SummarizeCommand) formatJSON(content string) string {
	data := map[string]interface{}{
		"focus":     c.Focus,
		"files":     c.Files,
		"summary":   content,
		"format":    "json",
		"timestamp": c.startTime.Format("2006-01-02T15:04:05Z07:00"),
		"brief":     c.Brief,
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		// Fallback to simple formatting if JSON marshaling fails
		return fmt.Sprintf(`{"error": "Failed to format JSON: %s"}`, err.Error())
	}
	return string(jsonBytes)
}

// formatHTML formats content as HTML
func (c *SummarizeCommand) formatHTML(content string) string {
	var result strings.Builder

	result.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	result.WriteString("<title>Code Summary</title>\n")
	result.WriteString("<style>body{font-family:Arial,sans-serif;margin:40px;}</style>\n")
	result.WriteString("</head>\n<body>\n")

	result.WriteString("<h1>Code Summary</h1>\n")

	if c.Focus != "" {
		result.WriteString(fmt.Sprintf("<p><strong>Focus:</strong> %s</p>\n", c.Focus))
	}

	result.WriteString("<h2>Files Analyzed</h2>\n<ul>\n")
	for _, file := range c.Files {
		result.WriteString(fmt.Sprintf("<li><code>%s</code></li>\n", file))
	}
	result.WriteString("</ul>\n")

	result.WriteString("<h2>Summary</h2>\n")
	result.WriteString("<div>\n")
	// Simple conversion - in real implementation, you'd properly escape HTML
	result.WriteString(strings.ReplaceAll(content, "\n", "<br>\n"))
	result.WriteString("\n</div>\n")

	result.WriteString("</body>\n</html>\n")

	return result.String()
}

// formatYAML formats content as YAML
func (c *SummarizeCommand) formatYAML(content string) string {
	var result strings.Builder

	result.WriteString("summary:\n")
	result.WriteString(fmt.Sprintf("  focus: \"%s\"\n", c.Focus))
	result.WriteString("  files:\n")
	for _, file := range c.Files {
		result.WriteString(fmt.Sprintf("    - \"%s\"\n", file))
	}
	result.WriteString(fmt.Sprintf("  brief: %t\n", c.Brief))
	result.WriteString(fmt.Sprintf("  timestamp: \"%s\"\n", c.startTime.Format("2006-01-02T15:04:05Z07:00")))
	result.WriteString("  content: |\n")

	// Indent content for YAML
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		result.WriteString(fmt.Sprintf("    %s\n", line))
	}

	return result.String()
}

// detectProjectLanguage detects the primary language of the project
func (c *SummarizeCommand) detectProjectLanguage() string {
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
func (c *SummarizeCommand) detectFramework() string {
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
func (c *SummarizeCommand) detectLanguage(filePath string) string {
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

// CreateCobraCommand creates the cobra command for summarize
func (c *SummarizeCommand) CreateCobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize [files...]",
		Short: "Generate code summaries with AI analysis",
		Long: `Generate comprehensive summaries of code files and projects using AI analysis.

The summarize command analyzes code structure, patterns, and functionality to provide
insightful summaries. It can focus on specific aspects and output in various formats.

Examples:
  sigil summarize main.go
  sigil summarize src/ --brief --focus "error handling"
  sigil summarize *.go --format html --output summary.html
  sigil summarize project/ --recursive --format yaml`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Files = args
			ctx := cmd.Context()
			return c.Execute(ctx)
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&c.Recursive, "recursive", "r", false, "Recursively summarize directories")
	cmd.Flags().BoolVar(&c.Brief, "brief", false, "Generate brief, high-level summary")
	cmd.Flags().StringVar(&c.Focus, "focus", "", "Focus area for summarization")
	cmd.Flags().StringVar(&c.Format, "format", "markdown", "Output format (markdown, text, json, html, yaml)")
	cmd.Flags().StringVarP(&c.OutputFile, "output", "o", "", "Output file (default: stdout)")

	return cmd
}

// fileExists checks if a file exists
func (c *SummarizeCommand) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readFile reads a file's content
func (c *SummarizeCommand) readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// writeFile writes content to a file
func (c *SummarizeCommand) writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

// Legacy summarize command for backwards compatibility
var summarizeCmd = NewSummarizeCommand().CreateCobraCommand()

// init registers the command during package initialization
func init() {
	// This ensures the command is available when root.go calls it
	summarizeCmd = NewSummarizeCommand().CreateCobraCommand()
}
