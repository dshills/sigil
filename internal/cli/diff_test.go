package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dshills/sigil/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDiffCommand(t *testing.T) {
	cmd := NewDiffCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "diff", cmd.BaseCommand.Name)
	assert.Equal(t, "markdown", cmd.Format)
	assert.Equal(t, 3, cmd.Context)
	assert.NotZero(t, cmd.startTime)
}

func TestDiffCommand_CreateCobraCommand(t *testing.T) {
	cmd := NewDiffCommand()
	cobraCmd := cmd.CreateCobraCommand()

	assert.NotNil(t, cobraCmd)
	assert.Equal(t, "diff", cobraCmd.Use[:4])
	assert.Contains(t, cobraCmd.Short, "Analyze code differences")

	// Check flags
	assert.NotNil(t, cobraCmd.Flags().Lookup("staged"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("commit"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("branch"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("summary"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("detailed"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("format"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("output"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("context"))
}

func TestDiffCommand_buildDescription(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*DiffCommand)
		expected string
	}{
		{
			name:     "basic description",
			setup:    func(c *DiffCommand) {},
			expected: "Analyze git diff and explain code changes",
		},
		{
			name: "with commit",
			setup: func(c *DiffCommand) {
				c.Commit = "abc123"
			},
			expected: "Analyze git diff and explain code changes for commit abc123",
		},
		{
			name: "with branch",
			setup: func(c *DiffCommand) {
				c.Branch = "main"
			},
			expected: "Analyze git diff and explain code changes compared to branch main",
		},
		{
			name: "with staged",
			setup: func(c *DiffCommand) {
				c.Staged = true
			},
			expected: "Analyze git diff and explain code changes for staged changes",
		},
		{
			name: "with files",
			setup: func(c *DiffCommand) {
				c.Files = []string{"file1.go", "file2.go"}
			},
			expected: "Analyze git diff and explain code changes for files: file1.go, file2.go",
		},
		{
			name: "with summary",
			setup: func(c *DiffCommand) {
				c.Summary = true
			},
			expected: "Analyze git diff and explain code changes (summary requested)",
		},
		{
			name: "with detailed",
			setup: func(c *DiffCommand) {
				c.Detailed = true
			},
			expected: "Analyze git diff and explain code changes (detailed analysis requested)",
		},
		{
			name: "with all options",
			setup: func(c *DiffCommand) {
				c.Commit = "xyz789"
				c.Summary = true
				c.Detailed = true
			},
			expected: "Analyze git diff and explain code changes for commit xyz789 (summary requested) (detailed analysis requested)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDiffCommand()
			tt.setup(cmd)
			result := cmd.buildDescription()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDiffCommand_createDiffTask(t *testing.T) {
	cmd := NewDiffCommand()
	cmd.Summary = true
	cmd.Detailed = true
	cmd.Format = "json"

	diffContent := "diff --git a/file.go b/file.go\n+added line\n-removed line"

	task, err := cmd.createDiffTask(diffContent)
	require.NoError(t, err)
	assert.NotNil(t, task)

	// Check task properties
	assert.Contains(t, task.ID, "diff_")
	assert.Equal(t, agent.TaskTypeAnalyze, task.Type)
	assert.Equal(t, agent.PriorityMedium, task.Priority)

	// Check file context
	assert.Len(t, task.Context.Files, 1)
	assert.Equal(t, "diff.patch", task.Context.Files[0].Path)
	assert.Equal(t, diffContent, task.Context.Files[0].Content)
	assert.Equal(t, "diff", task.Context.Files[0].Language)
	assert.True(t, task.Context.Files[0].IsTarget)

	// Check requirements
	assert.Contains(t, task.Context.Requirements, "Provide a concise summary of the changes")
	assert.Contains(t, task.Context.Requirements, "Provide detailed line-by-line analysis")
	assert.Contains(t, task.Context.Requirements, "Format the analysis as json")
}

func TestDiffCommand_formatMarkdown(t *testing.T) {
	cmd := NewDiffCommand()
	cmd.Commit = "abc123"
	cmd.Summary = false

	analysis := "This diff shows important changes"
	diffContent := "diff content here"

	result := cmd.formatMarkdown(analysis, diffContent)

	assert.Contains(t, result, "# Diff Analysis")
	assert.Contains(t, result, "**Commit:** abc123")
	assert.Contains(t, result, "## Analysis")
	assert.Contains(t, result, analysis)
	assert.Contains(t, result, "## Diff Content")
	assert.Contains(t, result, "```diff")
	assert.Contains(t, result, diffContent)
}

func TestDiffCommand_formatText(t *testing.T) {
	cmd := NewDiffCommand()
	cmd.Branch = "main"

	analysis := "Analysis content"
	diffContent := "diff content"

	result := cmd.formatText(analysis, diffContent)

	assert.Contains(t, result, "DIFF ANALYSIS")
	assert.Contains(t, result, "Compared to branch: main")
	assert.Contains(t, result, "Analysis:")
	assert.Contains(t, result, analysis)
	assert.Contains(t, result, "Diff Content:")
	assert.Contains(t, result, diffContent)
}

func TestDiffCommand_formatJSON(t *testing.T) {
	cmd := NewDiffCommand()
	cmd.Commit = "abc123"
	cmd.Summary = true
	cmd.Detailed = false
	cmd.startTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	analysis := "Test analysis"
	diffContent := "Test diff"

	result := cmd.formatJSON(analysis, diffContent)

	// Parse result to verify it's valid JSON
	assert.Contains(t, result, `"type": "commit"`)
	assert.Contains(t, result, `"reference": "abc123"`)
	assert.Contains(t, result, `"summary": true`)
	assert.Contains(t, result, `"detailed": false`)
	assert.Contains(t, result, `"analysis": "Test analysis"`)
	assert.Contains(t, result, `"diff_content": "Test diff"`)
}

func TestDiffCommand_formatHTML(t *testing.T) {
	cmd := NewDiffCommand()
	cmd.Staged = true

	analysis := "HTML analysis"
	diffContent := "HTML diff"

	result := cmd.formatHTML(analysis, diffContent)

	assert.Contains(t, result, "<!DOCTYPE html>")
	assert.Contains(t, result, "<title>Diff Analysis</title>")
	assert.Contains(t, result, "<strong>Type:</strong> Staged changes")
	assert.Contains(t, result, "<h2>Analysis</h2>")
	assert.Contains(t, result, analysis)
	assert.Contains(t, result, "<pre><code>")
	assert.Contains(t, result, diffContent)
}

func TestDiffCommand_formatOutput(t *testing.T) {
	tests := []struct {
		format   string
		analysis string
		diff     string
		check    func(t *testing.T, result string)
	}{
		{
			format:   "markdown",
			analysis: "md analysis",
			diff:     "md diff",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "# Diff Analysis")
			},
		},
		{
			format:   "text",
			analysis: "text analysis",
			diff:     "text diff",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "DIFF ANALYSIS")
			},
		},
		{
			format:   "json",
			analysis: "json analysis",
			diff:     "json diff",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "{")
				assert.Contains(t, result, "}")
			},
		},
		{
			format:   "html",
			analysis: "html analysis",
			diff:     "html diff",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "<!DOCTYPE html>")
			},
		},
		{
			format:   "unknown",
			analysis: "raw analysis",
			diff:     "raw diff",
			check: func(t *testing.T, result string) {
				assert.Equal(t, "raw analysis", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			cmd := NewDiffCommand()
			cmd.Format = tt.format

			result, err := cmd.formatOutput(tt.analysis, tt.diff)
			require.NoError(t, err)
			tt.check(t, result)
		})
	}
}

func TestDiffCommand_detectProjectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]bool
		expected string
	}{
		{
			name:     "Go project",
			files:    map[string]bool{"go.mod": true},
			expected: "go",
		},
		{
			name:     "JavaScript project",
			files:    map[string]bool{"package.json": true},
			expected: "javascript",
		},
		{
			name:     "Python project",
			files:    map[string]bool{"requirements.txt": true},
			expected: "python",
		},
		{
			name:     "Java project",
			files:    map[string]bool{"pom.xml": true},
			expected: "java",
		},
		{
			name:     "Unknown project",
			files:    map[string]bool{},
			expected: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require mocking fileExists
			// TODO: Refactor to use actual temporary files instead of mocking
			t.Skip("Skipping test that requires mocking fileExists method")
		})
	}
}

func TestDiffCommand_detectFramework(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]bool
		expected string
	}{
		{
			name:     "Next.js",
			files:    map[string]bool{"next.config.js": true},
			expected: "next.js",
		},
		{
			name:     "Angular",
			files:    map[string]bool{"angular.json": true},
			expected: "angular",
		},
		{
			name:     "Vue",
			files:    map[string]bool{"vue.config.js": true},
			expected: "vue",
		},
		{
			name:     "No framework",
			files:    map[string]bool{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require mocking fileExists
			// TODO: Refactor to use actual temporary files instead of mocking
			t.Skip("Skipping test that requires mocking fileExists method")
		})
	}
}

func TestDiffCommand_fileOperations(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	cmd := NewDiffCommand()

	t.Run("fileExists", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tmpDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		assert.True(t, cmd.fileExists(testFile))
		assert.False(t, cmd.fileExists(filepath.Join(tmpDir, "nonexistent.txt")))
	})

	t.Run("writeFile", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "output.txt")
		content := "test content"

		err := cmd.writeFile(testFile, content)
		require.NoError(t, err)

		// Read back and verify
		data, err := os.ReadFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, content, string(data))

		// Check permissions
		info, err := os.Stat(testFile)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})
}

func TestDiffCommand_Execute_Validation(t *testing.T) {
	// Create a temporary directory that's not a git repo
	tmpDir := t.TempDir()
	
	// Change to the temporary directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)
	
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	
	cmd := NewDiffCommand()
	ctx := context.Background()

	// Should fail outside git repository
	err = cmd.Execute(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git")
}

func TestDiffCommand_getCommitDiff(t *testing.T) {
	// Skip this test as it requires a real git repository with commits
	// TODO: Create a test git repository with known commits
	t.Skip("Skipping test that requires a git repository with commits")
}

func TestDiffCommand_getBranchDiff(t *testing.T) {
	// Skip this test as it requires a real git repository with branches
	// TODO: Create a test git repository with known branches
	t.Skip("Skipping test that requires a git repository with branches")
}

func TestDiffCommand_getFileDiff(t *testing.T) {
	// Skip this test as it requires a real git repository
	// TODO: Create a test git repository with test files
	t.Skip("Skipping test that requires a git repository")
}
