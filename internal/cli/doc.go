// Package cli provides command-line interface implementations
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dshills/sigil/internal/agent"
	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
)

// DocCommand handles documentation generation operations
type DocCommand struct {
	*BaseCommand
	Files          []string
	OutputDir      string
	Format         string
	Template       string
	IncludePrivate bool
	IncludeTests   bool
	Recursive      bool
	UpdateExisting bool
	Language       string
	startTime      time.Time
}

// NewDocCommand creates a new doc command
func NewDocCommand() *DocCommand {
	return &DocCommand{
		BaseCommand: NewBaseCommand("doc", "Generate documentation with AI assistance",
			"Generate comprehensive documentation for code files and projects using AI analysis."),
		Format:    "markdown",
		OutputDir: "docs",
		startTime: time.Now(),
	}
}

// Execute runs the doc command
func (c *DocCommand) Execute(ctx context.Context) error {
	logger.Info("starting documentation generation", "files", c.Files, "format", c.Format, "output_dir", c.OutputDir)

	// Validate inputs
	if err := c.validateInputs(); err != nil {
		return err
	}

	// Ensure output directory exists
	if err := c.ensureOutputDir(); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "Execute", "failed to create output directory")
	}

	// Process files for documentation
	fileContexts, err := c.processFiles()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInput, "Execute", "failed to process files")
	}

	// Create task for agent processing
	task, err := c.createDocTask(fileContexts)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to create doc task")
	}

	// Execute documentation generation
	result, err := c.executeDocGeneration(ctx, task)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to execute documentation generation")
	}

	// Output documentation files
	return c.outputDocumentation(result)
}

// validateInputs validates the command inputs
func (c *DocCommand) validateInputs() error {
	if len(c.Files) == 0 {
		return errors.New(errors.ErrorTypeInput, "validateInputs", "no files specified for documentation")
	}

	for _, file := range c.Files {
		if !c.fileExists(file) {
			return errors.New(errors.ErrorTypeInput, "validateInputs",
				fmt.Sprintf("file does not exist: %s", file))
		}
	}

	validFormats := []string{FormatMarkdown, FormatHTML, "rst", "asciidoc", "text"}
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

// ensureOutputDir creates the output directory if it doesn't exist
func (c *DocCommand) ensureOutputDir() error {
	return os.MkdirAll(c.OutputDir, 0755)
}

// processFiles reads and processes all specified files
func (c *DocCommand) processFiles() ([]agent.FileContext, error) {
	fileContexts := make([]agent.FileContext, 0, len(c.Files))

	for _, filePath := range c.Files {
		// Skip test files if not including tests
		if !c.IncludeTests && c.isTestFile(filePath) {
			continue
		}

		content, err := c.readFile(filePath)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrorTypeInput, "processFiles",
				fmt.Sprintf("failed to read file: %s", filePath))
		}

		// Determine if file contains private/internal code
		hasPrivate := c.hasPrivateContent(content)
		if !c.IncludePrivate && hasPrivate {
			// Filter out private content
			content = c.filterPrivateContent(content)
		}

		fileContext := agent.FileContext{
			Path:        filePath,
			Content:     content,
			Language:    c.detectLanguage(filePath),
			Purpose:     "Code to document",
			IsTarget:    true,
			IsReference: false,
		}
		fileContexts = append(fileContexts, fileContext)
	}

	return fileContexts, nil
}

// isTestFile checks if a file is a test file
func (c *DocCommand) isTestFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	return strings.Contains(fileName, "_test.") ||
		strings.Contains(fileName, ".test.") ||
		strings.HasSuffix(fileName, "_test.go") ||
		strings.HasSuffix(fileName, ".spec.js") ||
		strings.HasSuffix(fileName, ".spec.ts") ||
		strings.Contains(filePath, "/test/") ||
		strings.Contains(filePath, "/tests/")
}

// hasPrivateContent checks if content contains private/internal elements
func (c *DocCommand) hasPrivateContent(content string) bool {
	// Simple heuristic - can be enhanced based on language
	return strings.Contains(content, "private ") ||
		strings.Contains(content, "internal ") ||
		strings.Contains(content, "// private") ||
		strings.Contains(content, "# private")
}

// filterPrivateContent removes private content from documentation
func (c *DocCommand) filterPrivateContent(content string) string {
	// Simple implementation - in practice, this would be more sophisticated
	lines := strings.Split(content, "\n")
	var filtered []string

	for _, line := range lines {
		if !strings.Contains(line, "private ") &&
			!strings.Contains(line, "internal ") {
			filtered = append(filtered, line)
		}
	}

	return strings.Join(filtered, "\n")
}

// createDocTask creates a task for documentation generation
func (c *DocCommand) createDocTask(fileContexts []agent.FileContext) (*agent.Task, error) {
	// Create requirements based on flags
	requirements := []string{
		"Generate comprehensive documentation for the provided code",
		"Include detailed explanations of functions, classes, and modules",
		"Explain usage patterns and provide examples where appropriate",
		"Document API interfaces and public methods",
		"Include architectural overview and design decisions",
	}

	if c.IncludePrivate {
		requirements = append(requirements, "Include documentation for private and internal components")
	}

	if c.IncludeTests {
		requirements = append(requirements, "Document test files and testing patterns")
	}

	requirements = append(requirements, fmt.Sprintf("Format the documentation as %s", c.Format))

	if c.Template != "" {
		requirements = append(requirements, fmt.Sprintf("Use the template style: %s", c.Template))
	}

	// Detect project info
	projectInfo := agent.ProjectInfo{
		Language:  c.detectProjectLanguage(),
		Framework: c.detectFramework(),
		Style:     "standard",
	}

	// Create task
	task := &agent.Task{
		ID:          fmt.Sprintf("doc_%d", c.startTime.Unix()),
		Type:        agent.TaskTypeGenerate,
		Description: c.buildDescription(),
		Context: agent.TaskContext{
			Files:        fileContexts,
			Requirements: requirements,
			ProjectInfo:  projectInfo,
		},
		Priority:  agent.PriorityLow,
		CreatedAt: c.startTime,
	}

	return task, nil
}

// buildDescription builds the task description
func (c *DocCommand) buildDescription() string {
	description := "Generate comprehensive documentation for the provided code files"

	if c.Language != "" {
		description += fmt.Sprintf(" (language: %s)", c.Language)
	}

	if c.Template != "" {
		description += fmt.Sprintf(" using template: %s", c.Template)
	}

	return description
}

// executeDocGeneration executes the documentation generation using the agent system
func (c *DocCommand) executeDocGeneration(ctx context.Context, task *agent.Task) (*agent.OrchestrationResult, error) {
	logger.Info("executing documentation generation with agent system")

	// Create agent factory and orchestrator
	factory := agent.NewFactory(nil, agent.DefaultOrchestrationConfig()) // No sandbox needed for documentation
	orchestrator, err := factory.CreateOrchestrator()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "executeDocGeneration", "failed to create orchestrator")
	}

	// Execute task
	result, err := orchestrator.ExecuteTask(ctx, *task)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "executeDocGeneration", "task execution failed")
	}

	if result.Status != agent.StatusSuccess {
		return nil, errors.New(errors.ErrorTypeInternal, "executeDocGeneration",
			fmt.Sprintf("documentation generation failed with status: %s", result.Status))
	}

	logger.Info("documentation generation completed", "status", result.Status)
	return result, nil
}

// outputDocumentation writes the generated documentation to files
func (c *DocCommand) outputDocumentation(result *agent.OrchestrationResult) error {
	if result.FinalResult == nil {
		return errors.New(errors.ErrorTypeInternal, "outputDocumentation", "no final result available")
	}

	// Check if we have artifacts (individual file documentation)
	if len(result.FinalResult.Artifacts) > 0 {
		for _, artifact := range result.FinalResult.Artifacts {
			if err := c.writeDocFile(artifact); err != nil {
				logger.Warn("failed to write doc file", "artifact", artifact.Name, "error", err)
			}
		}
	}

	// Write main documentation from reasoning
	if result.FinalResult.Reasoning != "" {
		mainDocFile := filepath.Join(c.OutputDir, fmt.Sprintf("README.%s", c.getFileExtension()))
		if err := c.writeFile(mainDocFile, result.FinalResult.Reasoning); err != nil {
			return errors.Wrap(err, errors.ErrorTypeFS, "outputDocumentation", "failed to write main documentation")
		}
		fmt.Printf("Main documentation written to: %s\n", mainDocFile)
	}

	fmt.Printf("Documentation generated in: %s\n", c.OutputDir)
	return nil
}

// writeDocFile writes a documentation artifact to a file
func (c *DocCommand) writeDocFile(artifact agent.Artifact) error {
	fileName := artifact.Name
	if !strings.Contains(fileName, ".") {
		fileName += "." + c.getFileExtension()
	}

	filePath := filepath.Join(c.OutputDir, fileName)

	// Check if file exists and UpdateExisting is false
	if !c.UpdateExisting && c.fileExists(filePath) {
		logger.Info("skipping existing file", "path", filePath)
		return nil
	}

	return c.writeFile(filePath, artifact.Content)
}

// getFileExtension returns the appropriate file extension for the format
func (c *DocCommand) getFileExtension() string {
	switch c.Format {
	case FormatMarkdown:
		return "md"
	case FormatHTML:
		return "html"
	case "rst":
		return "rst"
	case "asciidoc":
		return "adoc"
	case "text":
		return "txt"
	default:
		return "md"
	}
}

// detectProjectLanguage detects the primary language of the project
func (c *DocCommand) detectProjectLanguage() string {
	if c.Language != "" {
		return c.Language
	}

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
func (c *DocCommand) detectFramework() string {
	if c.fileExists("next.config.js") {
		return FrameworkNextJS
	}
	if c.fileExists("angular.json") {
		return FrameworkAngular
	}
	if c.fileExists("vue.config.js") {
		return FrameworkVue
	}
	return ""
}

// detectLanguage detects the language of a specific file
func (c *DocCommand) detectLanguage(filePath string) string {
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
	return "text"
}

// CreateCobraCommand creates the cobra command for doc
func (c *DocCommand) CreateCobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc [files...]",
		Short: "Generate documentation with AI assistance",
		Long: `Generate comprehensive documentation for code files and projects using AI analysis.

The doc command analyzes code structure, patterns, and functionality to generate
detailed documentation in various formats. It can include API references,
usage examples, and architectural overviews.

Examples:
  sigil doc main.go                              # Document a single file
  sigil doc src/                                 # Document all files in directory
  sigil doc *.go --format html --output docs/   # Generate HTML docs
  sigil doc project/ --include-private --template api`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Files = args
			ctx := cmd.Context()
			return c.Execute(ctx)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&c.OutputDir, "output", "docs", "Output directory for documentation")
	cmd.Flags().StringVar(&c.Format, "format", "markdown", "Output format (markdown,html,rst,asciidoc,text)")
	cmd.Flags().StringVar(&c.Template, "template", "", "Documentation template style")
	cmd.Flags().BoolVar(&c.IncludePrivate, "include-private", false, "Include private/internal components")
	cmd.Flags().BoolVar(&c.IncludeTests, "include-tests", false, "Include test files in documentation")
	cmd.Flags().BoolVarP(&c.Recursive, "recursive", "r", false, "Process directories recursively")
	cmd.Flags().BoolVar(&c.UpdateExisting, "update", false, "Update existing documentation files")
	cmd.Flags().StringVar(&c.Language, "language", "", "Override language detection")

	return cmd
}

// fileExists checks if a file exists
func (c *DocCommand) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readFile reads a file's content
func (c *DocCommand) readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// writeFile writes content to a file
func (c *DocCommand) writeFile(path, content string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0600)
}

// Legacy doc command for backwards compatibility
var docCmd = NewDocCommand().CreateCobraCommand()

// init registers the command during package initialization
func init() {
	// This ensures the command is available when root.go calls it
	docCmd = NewDocCommand().CreateCobraCommand()
}
