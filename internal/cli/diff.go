// Package cli provides command-line interface implementations
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dshills/sigil/internal/agent"
	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/git"
	"github.com/dshills/sigil/internal/logger"
)

// DiffCommand handles diff analysis operations
type DiffCommand struct {
	*BaseCommand
	Files      []string
	Staged     bool
	Commit     string
	Branch     string
	Summary    bool
	Detailed   bool
	Format     string
	OutputFile string
	Context    int
	startTime  time.Time
}

// NewDiffCommand creates a new diff command
func NewDiffCommand() *DiffCommand {
	return &DiffCommand{
		BaseCommand: NewBaseCommand("diff", "Analyze code differences with AI insights",
			"Analyze git diffs and code changes using AI-powered analysis to understand impact and implications."),
		Format:    "markdown",
		Context:   3,
		startTime: time.Now(),
	}
}

// Execute runs the diff command
func (c *DiffCommand) Execute(ctx context.Context) error {
	logger.Info("starting diff analysis", "files", c.Files, "staged", c.Staged, "commit", c.Commit)

	// Validate Git repository
	gitRepo, err := git.NewRepository(".")
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "Execute", "failed to open git repository")
	}

	// Get diff content
	diffContent, err := c.getDiffContent(gitRepo)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "Execute", "failed to get diff content")
	}

	if diffContent == "" {
		fmt.Println("No changes found")
		return nil
	}

	// Create task for agent processing
	task, err := c.createDiffTask(diffContent)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to create diff task")
	}

	// Execute analysis
	result, err := c.executeDiffAnalysis(ctx, task)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "Execute", "failed to execute diff analysis")
	}

	// Output result
	return c.outputResult(result, diffContent)
}

// getDiffContent retrieves diff content based on command options
func (c *DiffCommand) getDiffContent(gitRepo *git.Repository) (string, error) {
	var diffContent string
	var err error

	switch {
	case c.Commit != "":
		// Get diff for specific commit
		diffContent, err = c.getCommitDiff(gitRepo, c.Commit)
	case c.Branch != "":
		// Get diff between current branch and specified branch
		diffContent, err = c.getBranchDiff(gitRepo, c.Branch)
	case c.Staged:
		// Get staged changes
		diffContent, err = gitRepo.GetStagedDiff()
	case len(c.Files) > 0:
		// Get diff for specific files
		diffContent, err = c.getFileDiff(gitRepo, c.Files)
	default:
		// Get working directory changes
		diffContent, err = gitRepo.GetDiff()
	}

	if err != nil {
		return "", err
	}

	return diffContent, nil
}

// getCommitDiff gets diff for a specific commit
func (c *DiffCommand) getCommitDiff(gitRepo *git.Repository, commit string) (string, error) {
	// Validate commit parameter
	if commit == "" {
		return "", errors.New(errors.ErrorTypeInput, "getCommitDiff", "commit hash cannot be empty")
	}

	// Use git show to get the diff for a specific commit
	cmd := exec.Command("git", "show", "--format=", commit)
	cmd.Dir = gitRepo.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's a command not found error
		if strings.Contains(err.Error(), "executable file not found") {
			return "", errors.New(errors.ErrorTypeInternal, "getCommitDiff", "git command not found in PATH")
		}
		return "", errors.Wrap(err, errors.ErrorTypeGit, "getCommitDiff",
			fmt.Sprintf("failed to get diff for commit %s: %s", commit, string(output)))
	}

	return string(output), nil
}

// getBranchDiff gets diff between current branch and specified branch
func (c *DiffCommand) getBranchDiff(gitRepo *git.Repository, branch string) (string, error) {
	// Validate branch parameter
	if branch == "" {
		return "", errors.New(errors.ErrorTypeInput, "getBranchDiff", "branch name cannot be empty")
	}

	// Get diff between the specified branch and current HEAD
	// Using three dots (...) to show changes on HEAD since the branches diverged
	cmd := exec.Command("git", "diff", fmt.Sprintf("%s...HEAD", branch))
	cmd.Dir = gitRepo.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's a command not found error
		if strings.Contains(err.Error(), "executable file not found") {
			return "", errors.New(errors.ErrorTypeInternal, "getBranchDiff", "git command not found in PATH")
		}

		// Check if the branch exists
		checkCmd := exec.Command("git", "rev-parse", "--verify", branch)
		checkCmd.Dir = gitRepo.Path
		if checkErr := checkCmd.Run(); checkErr != nil {
			return "", errors.New(errors.ErrorTypeInput, "getBranchDiff",
				fmt.Sprintf("branch '%s' does not exist", branch))
		}
		return "", errors.Wrap(err, errors.ErrorTypeGit, "getBranchDiff",
			fmt.Sprintf("failed to get diff for branch %s: %s", branch, string(output)))
	}

	return string(output), nil
}

// getFileDiff gets diff for specific files
func (c *DiffCommand) getFileDiff(gitRepo *git.Repository, files []string) (string, error) {
	// Use the existing Diff method from the git package
	opts := git.DiffOptions{
		Staged: c.Staged,
		Files:  files,
	}

	diffContent, err := gitRepo.Diff(opts)
	if err != nil {
		return "", errors.Wrap(err, errors.ErrorTypeGit, "getFileDiff",
			fmt.Sprintf("failed to get diff for files %v", files))
	}

	// Check if any of the files don't exist
	if diffContent == "" {
		for _, file := range files {
			if !c.fileExists(file) {
				return "", errors.New(errors.ErrorTypeInput, "getFileDiff",
					fmt.Sprintf("file does not exist: %s", file))
			}
		}
		// Files exist but no changes
		return "", nil
	}

	return diffContent, nil
}

// createDiffTask creates a task for diff analysis
func (c *DiffCommand) createDiffTask(diffContent string) (*agent.Task, error) {
	// Create file context for the diff
	fileContext := agent.FileContext{
		Path:        "diff.patch",
		Content:     diffContent,
		Language:    "diff",
		Purpose:     "Git diff to analyze",
		IsTarget:    true,
		IsReference: false,
	}

	// Create requirements based on flags
	requirements := []string{
		"Analyze the git diff and explain the changes",
		"Identify the impact and implications of the changes",
		"Highlight potential issues or risks",
		"Explain the purpose and context of the modifications",
	}

	if c.Summary {
		requirements = append(requirements, "Provide a concise summary of the changes")
	}

	if c.Detailed {
		requirements = append(requirements,
			"Provide detailed line-by-line analysis",
			"Explain complex changes and their interactions",
			"Include suggestions for improvement where applicable")
	}

	requirements = append(requirements, fmt.Sprintf("Format the analysis as %s", c.Format))

	// Detect project info
	projectInfo := agent.ProjectInfo{
		Language:  c.detectProjectLanguage(),
		Framework: c.detectFramework(),
		Style:     "standard",
	}

	// Create task
	task := &agent.Task{
		ID:          fmt.Sprintf("diff_%d", c.startTime.Unix()),
		Type:        agent.TaskTypeAnalyze,
		Description: c.buildDescription(),
		Context: agent.TaskContext{
			Files:        []agent.FileContext{fileContext},
			Requirements: requirements,
			ProjectInfo:  projectInfo,
		},
		Priority:  agent.PriorityMedium,
		CreatedAt: c.startTime,
	}

	return task, nil
}

// buildDescription builds the task description
func (c *DiffCommand) buildDescription() string {
	description := "Analyze git diff and explain code changes"

	if c.Commit != "" {
		description += fmt.Sprintf(" for commit %s", c.Commit)
	} else if c.Branch != "" {
		description += fmt.Sprintf(" compared to branch %s", c.Branch)
	} else if c.Staged {
		description += " for staged changes"
	} else if len(c.Files) > 0 {
		description += fmt.Sprintf(" for files: %s", strings.Join(c.Files, ", "))
	}

	if c.Summary {
		description += " (summary requested)"
	}

	if c.Detailed {
		description += " (detailed analysis requested)"
	}

	return description
}

// executeDiffAnalysis executes the diff analysis using the agent system
func (c *DiffCommand) executeDiffAnalysis(ctx context.Context, task *agent.Task) (*agent.OrchestrationResult, error) {
	logger.Info("executing diff analysis with agent system")

	// Create agent factory and orchestrator
	factory := agent.NewFactory(nil, agent.DefaultOrchestrationConfig()) // No sandbox needed for diff analysis
	orchestrator, err := factory.CreateOrchestrator()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "executeDiffAnalysis", "failed to create orchestrator")
	}

	// Execute task
	result, err := orchestrator.ExecuteTask(ctx, *task)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "executeDiffAnalysis", "task execution failed")
	}

	if result.Status != agent.StatusSuccess {
		return nil, errors.New(errors.ErrorTypeInternal, "executeDiffAnalysis",
			fmt.Sprintf("diff analysis failed with status: %s", result.Status))
	}

	logger.Info("diff analysis completed", "status", result.Status)
	return result, nil
}

// outputResult outputs the diff analysis result
func (c *DiffCommand) outputResult(result *agent.OrchestrationResult, diffContent string) error {
	if result.FinalResult == nil {
		return errors.New(errors.ErrorTypeInternal, "outputResult", "no final result available")
	}

	analysis := result.FinalResult.Reasoning
	if analysis == "" && len(result.FinalResult.Artifacts) > 0 {
		// Use artifact content if reasoning is empty
		analysis = result.FinalResult.Artifacts[0].Content
	}

	if analysis == "" {
		return errors.New(errors.ErrorTypeInternal, "outputResult", "no analysis content generated")
	}

	// Format the output
	formatted, err := c.formatOutput(analysis, diffContent)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "outputResult", "failed to format output")
	}

	// Write to file or stdout
	if c.OutputFile != "" {
		if err := c.writeFile(c.OutputFile, formatted); err != nil {
			return errors.Wrap(err, errors.ErrorTypeInternal, "outputResult",
				fmt.Sprintf("failed to write output file: %s", c.OutputFile))
		}
		fmt.Printf("Diff analysis written to: %s\n", c.OutputFile)
	} else {
		fmt.Print(formatted)
	}

	return nil
}

// formatOutput formats the diff analysis based on the requested format
func (c *DiffCommand) formatOutput(analysis, diffContent string) (string, error) {
	switch c.Format {
	case "markdown":
		return c.formatMarkdown(analysis, diffContent), nil
	case "text":
		return c.formatText(analysis, diffContent), nil
	case "json":
		return c.formatJSON(analysis, diffContent), nil
	case "html":
		return c.formatHTML(analysis, diffContent), nil
	default:
		return analysis, nil
	}
}

// formatMarkdown formats content as markdown
func (c *DiffCommand) formatMarkdown(analysis, diffContent string) string {
	var output strings.Builder

	output.WriteString("# Diff Analysis\n\n")

	// Add context information
	if c.Commit != "" {
		output.WriteString(fmt.Sprintf("**Commit:** %s\n", c.Commit))
	} else if c.Branch != "" {
		output.WriteString(fmt.Sprintf("**Compared to branch:** %s\n", c.Branch))
	} else if c.Staged {
		output.WriteString("**Type:** Staged changes\n")
	} else if len(c.Files) > 0 {
		output.WriteString(fmt.Sprintf("**Files:** %s\n", strings.Join(c.Files, ", ")))
	} else {
		output.WriteString("**Type:** Working directory changes\n")
	}
	output.WriteString("\n")

	output.WriteString("## Analysis\n\n")
	output.WriteString(analysis)
	output.WriteString("\n\n")

	if !c.Summary {
		output.WriteString("## Diff Content\n\n")
		output.WriteString("```diff\n")
		output.WriteString(diffContent)
		output.WriteString("\n```\n")
	}

	return output.String()
}

// formatText formats content as plain text
func (c *DiffCommand) formatText(analysis, diffContent string) string {
	var output strings.Builder

	output.WriteString("DIFF ANALYSIS\n")
	output.WriteString("=============\n\n")

	// Add context information
	if c.Commit != "" {
		output.WriteString(fmt.Sprintf("Commit: %s\n", c.Commit))
	} else if c.Branch != "" {
		output.WriteString(fmt.Sprintf("Compared to branch: %s\n", c.Branch))
	} else if c.Staged {
		output.WriteString("Type: Staged changes\n")
	} else if len(c.Files) > 0 {
		output.WriteString(fmt.Sprintf("Files: %s\n", strings.Join(c.Files, ", ")))
	} else {
		output.WriteString("Type: Working directory changes\n")
	}
	output.WriteString("\n")

	output.WriteString("Analysis:\n")
	output.WriteString("---------\n")
	output.WriteString(analysis)
	output.WriteString("\n\n")

	if !c.Summary {
		output.WriteString("Diff Content:\n")
		output.WriteString("-------------\n")
		output.WriteString(diffContent)
		output.WriteString("\n")
	}

	return output.String()
}

// formatJSON formats content as JSON
func (c *DiffCommand) formatJSON(analysis, diffContent string) string {
	diffType := "working"
	reference := ""

	if c.Commit != "" {
		diffType = "commit"
		reference = c.Commit
	} else if c.Branch != "" {
		diffType = "branch"
		reference = c.Branch
	} else if c.Staged {
		diffType = "staged"
	} else if len(c.Files) > 0 {
		diffType = "files"
		reference = strings.Join(c.Files, ",")
	}

	data := map[string]interface{}{
		"diff_analysis": map[string]interface{}{
			"type":         diffType,
			"reference":    reference,
			"timestamp":    c.startTime.Format("2006-01-02T15:04:05Z07:00"),
			"summary":      c.Summary,
			"detailed":     c.Detailed,
			"analysis":     analysis,
			"diff_content": diffContent,
		},
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		// Fallback to simple formatting if JSON marshaling fails
		return fmt.Sprintf(`{"error": "Failed to format JSON: %s"}`, err.Error())
	}
	return string(jsonBytes)
}

// formatHTML formats content as HTML
func (c *DiffCommand) formatHTML(analysis, diffContent string) string {
	var output strings.Builder

	output.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	output.WriteString("<title>Diff Analysis</title>\n")
	output.WriteString("<style>body{font-family:Arial,sans-serif;margin:40px;}pre{background:#f5f5f5;padding:10px;}</style>\n")
	output.WriteString("</head>\n<body>\n")

	output.WriteString("<h1>Diff Analysis</h1>\n")

	// Add context information
	if c.Commit != "" {
		output.WriteString(fmt.Sprintf("<p><strong>Commit:</strong> %s</p>\n", c.Commit))
	} else if c.Branch != "" {
		output.WriteString(fmt.Sprintf("<p><strong>Compared to branch:</strong> %s</p>\n", c.Branch))
	} else if c.Staged {
		output.WriteString("<p><strong>Type:</strong> Staged changes</p>\n")
	} else if len(c.Files) > 0 {
		output.WriteString(fmt.Sprintf("<p><strong>Files:</strong> %s</p>\n", strings.Join(c.Files, ", ")))
	} else {
		output.WriteString("<p><strong>Type:</strong> Working directory changes</p>\n")
	}

	output.WriteString("<h2>Analysis</h2>\n")
	output.WriteString("<div>\n")
	output.WriteString(strings.ReplaceAll(analysis, "\n", "<br>\n"))
	output.WriteString("\n</div>\n")

	if !c.Summary {
		output.WriteString("<h2>Diff Content</h2>\n")
		output.WriteString("<pre><code>\n")
		output.WriteString(diffContent)
		output.WriteString("\n</code></pre>\n")
	}

	output.WriteString("</body>\n</html>\n")

	return output.String()
}

// detectProjectLanguage detects the primary language of the project
func (c *DiffCommand) detectProjectLanguage() string {
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
func (c *DiffCommand) detectFramework() string {
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

// CreateCobraCommand creates the cobra command for diff
func (c *DiffCommand) CreateCobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff [files...]",
		Short: "Analyze code differences with AI insights",
		Long: `Analyze git diffs and code changes using AI-powered analysis to understand impact and implications.

The diff command can analyze various types of changes including working directory changes,
staged changes, specific commits, or comparisons between branches.

Examples:
  sigil diff                              # Analyze working directory changes
  sigil diff --staged                     # Analyze staged changes
  sigil diff --commit abc123              # Analyze specific commit
  sigil diff --branch main                # Compare current branch to main
  sigil diff file1.go file2.go           # Analyze specific files
  sigil diff --summary --format json     # Get summary in JSON format`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c.Files = args
			ctx := cmd.Context()
			return c.Execute(ctx)
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&c.Staged, "staged", false, "Analyze staged changes")
	cmd.Flags().StringVar(&c.Commit, "commit", "", "Analyze specific commit")
	cmd.Flags().StringVar(&c.Branch, "branch", "", "Compare against specific branch")
	cmd.Flags().BoolVar(&c.Summary, "summary", false, "Provide summary only (exclude diff content)")
	cmd.Flags().BoolVar(&c.Detailed, "detailed", false, "Provide detailed line-by-line analysis")
	cmd.Flags().StringVar(&c.Format, "format", "markdown", "Output format (markdown,text,json,html)")
	cmd.Flags().StringVarP(&c.OutputFile, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().IntVarP(&c.Context, "context", "C", 3, "Lines of context around changes")

	return cmd
}

// fileExists checks if a file exists
func (c *DiffCommand) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// writeFile writes content to a file
func (c *DiffCommand) writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

// Legacy diff command for backwards compatibility
var diffCmd = NewDiffCommand().CreateCobraCommand()

// init registers the command during package initialization
func init() {
	// This ensures the command is available when root.go calls it
	diffCmd = NewDiffCommand().CreateCobraCommand()
}
