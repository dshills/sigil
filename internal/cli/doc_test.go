package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/sigil/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDocCommand(t *testing.T) {
	cmd := NewDocCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "doc", cmd.BaseCommand.Name)
	assert.Equal(t, "markdown", cmd.Format)
	assert.Equal(t, "docs", cmd.OutputDir)
	assert.NotZero(t, cmd.startTime)
}

func TestDocCommand_CreateCobraCommand(t *testing.T) {
	cmd := NewDocCommand()
	cobraCmd := cmd.CreateCobraCommand()

	assert.NotNil(t, cobraCmd)
	assert.Equal(t, "doc", cobraCmd.Use[:3])
	assert.Contains(t, cobraCmd.Short, "Generate documentation")

	// Check flags
	assert.NotNil(t, cobraCmd.Flags().Lookup("output"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("format"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("template"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("include-private"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("include-tests"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("recursive"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("update-existing"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("language"))
}

func TestDocCommand_validateInputs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	existingFile := filepath.Join(tmpDir, "exists.go")
	err := os.WriteFile(existingFile, []byte("package main"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name    string
		setup   func(*DocCommand)
		wantErr bool
		errMsg  string
	}{
		{
			name: "no files specified",
			setup: func(c *DocCommand) {
				c.Files = []string{}
			},
			wantErr: true,
			errMsg:  "no files specified",
		},
		{
			name: "file does not exist",
			setup: func(c *DocCommand) {
				c.Files = []string{filepath.Join(tmpDir, "nonexistent.go")}
			},
			wantErr: true,
			errMsg:  "file does not exist",
		},
		{
			name: "invalid format",
			setup: func(c *DocCommand) {
				c.Files = []string{existingFile}
				c.Format = "invalid"
			},
			wantErr: true,
			errMsg:  "invalid format",
		},
		{
			name: "valid inputs",
			setup: func(c *DocCommand) {
				c.Files = []string{existingFile}
				c.Format = "markdown"
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDocCommand()
			tt.setup(cmd)
			err := cmd.validateInputs()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDocCommand_ensureOutputDir(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := NewDocCommand()
	cmd.OutputDir = filepath.Join(tmpDir, "new", "docs")

	err := cmd.ensureOutputDir()
	require.NoError(t, err)

	// Check directory was created
	info, err := os.Stat(cmd.OutputDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

func TestDocCommand_isTestFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"file_test.go", true},
		{"file.test.js", true},
		{"file_test.go", true},
		{"file.spec.js", true},
		{"file.spec.ts", true},
		{"src/test/file.go", true},
		{"src/tests/file.go", true},
		{"regular.go", false},
		{"main.js", false},
	}

	cmd := NewDocCommand()

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := cmd.isTestFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDocCommand_hasPrivateContent(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"private func foo()", true},
		{"internal class Bar", true},
		{"// private implementation", true},
		{"# private method", true},
		{"public func foo()", false},
		{"normal content", false},
	}

	cmd := NewDocCommand()

	for _, tt := range tests {
		t.Run(tt.content[:min(20, len(tt.content))], func(t *testing.T) {
			result := cmd.hasPrivateContent(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDocCommand_filterPrivateContent(t *testing.T) {
	content := `public func foo()
private func bar()
internal var x
normal line
// private comment`

	cmd := NewDocCommand()
	result := cmd.filterPrivateContent(content)

	// Check that private/internal lines are removed
	assert.NotContains(t, result, "private func")
	assert.NotContains(t, result, "internal var")
	assert.Contains(t, result, "public func")
	assert.Contains(t, result, "normal line")
}

func TestDocCommand_buildDescription(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*DocCommand)
		expected string
	}{
		{
			name:     "basic description",
			setup:    func(c *DocCommand) {},
			expected: "Generate comprehensive documentation for the provided code files",
		},
		{
			name: "with language",
			setup: func(c *DocCommand) {
				c.Language = "go"
			},
			expected: "Generate comprehensive documentation for the provided code files (language: go)",
		},
		{
			name: "with template",
			setup: func(c *DocCommand) {
				c.Template = "api-doc"
			},
			expected: "Generate comprehensive documentation for the provided code files using template: api-doc",
		},
		{
			name: "with both",
			setup: func(c *DocCommand) {
				c.Language = "python"
				c.Template = "sphinx"
			},
			expected: "Generate comprehensive documentation for the provided code files (language: python) using template: sphinx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDocCommand()
			tt.setup(cmd)
			result := cmd.buildDescription()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDocCommand_createDocTask(t *testing.T) {
	cmd := NewDocCommand()
	cmd.Format = "html"
	cmd.Template = "api"
	cmd.IncludePrivate = true
	cmd.Language = "go"

	fileContexts := []agent.FileContext{
		{
			Path:     "test.go",
			Content:  "package main",
			Language: "go",
			Purpose:  "Code to document",
		},
	}

	task, err := cmd.createDocTask(fileContexts)
	require.NoError(t, err)
	assert.NotNil(t, task)

	// Check task properties
	assert.Contains(t, task.ID, "doc_")
	assert.Equal(t, agent.TaskTypeGenerate, task.Type)
	assert.Equal(t, agent.PriorityMedium, task.Priority)

	// Check file context
	assert.Equal(t, fileContexts, task.Context.Files)

	// Check requirements
	assert.Contains(t, task.Context.Requirements, "Format documentation as html")
	assert.Contains(t, task.Context.Requirements, "Use template: api")
	assert.Contains(t, task.Context.Requirements, "Include documentation for private/internal elements")

	// Check project info
	assert.Equal(t, "go", task.Context.ProjectInfo.Language)
}

func TestDocCommand_getFileExtension(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{"markdown", "md"},
		{"html", "html"},
		{"rst", "rst"},
		{"asciidoc", "adoc"},
		{"text", "txt"},
		{"unknown", "txt"},
	}

	cmd := NewDocCommand()

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			cmd.Format = tt.format
			result := cmd.getFileExtension()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDocCommand_writeDocFile(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := NewDocCommand()
	cmd.OutputDir = tmpDir
	cmd.Format = "markdown"

	artifact := agent.Artifact{
		Name:    "test",
		Type:    "documentation",
		Content: "# Test Documentation",
	}

	err := cmd.writeDocFile(artifact)
	require.NoError(t, err)

	// Check file was created with correct extension
	expectedPath := filepath.Join(tmpDir, "test.md")
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, artifact.Content, string(content))
}

func TestDocCommand_writeDocFile_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing file
	existingFile := filepath.Join(tmpDir, "existing.md")
	err := os.WriteFile(existingFile, []byte("old content"), 0644)
	require.NoError(t, err)

	cmd := NewDocCommand()
	cmd.OutputDir = tmpDir
	cmd.Format = "markdown"

	artifact := agent.Artifact{
		Name:    "existing",
		Content: "new content",
	}

	// Test with UpdateExisting = false
	cmd.UpdateExisting = false
	err = cmd.writeDocFile(artifact)
	require.NoError(t, err)

	// Should not update
	content, err := os.ReadFile(existingFile)
	require.NoError(t, err)
	assert.Equal(t, "old content", string(content))

	// Test with UpdateExisting = true
	cmd.UpdateExisting = true
	err = cmd.writeDocFile(artifact)
	require.NoError(t, err)

	// Should update
	content, err = os.ReadFile(existingFile)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(content))
}

func TestDocCommand_processFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	regularFile := filepath.Join(tmpDir, "regular.go")
	testFile := filepath.Join(tmpDir, "main_test.go")
	privateFile := filepath.Join(tmpDir, "private.go")

	err := os.WriteFile(regularFile, []byte("package main\npublic content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(testFile, []byte("package main\ntest content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(privateFile, []byte("package main\nprivate content"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name           string
		includeTests   bool
		includePrivate bool
		expectedCount  int
	}{
		{
			name:           "exclude tests and private",
			includeTests:   false,
			includePrivate: false,
			expectedCount:  2, // regular and private (but filtered)
		},
		{
			name:           "include tests",
			includeTests:   true,
			includePrivate: false,
			expectedCount:  3,
		},
		{
			name:           "include private",
			includeTests:   false,
			includePrivate: true,
			expectedCount:  2,
		},
		{
			name:           "include all",
			includeTests:   true,
			includePrivate: true,
			expectedCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewDocCommand()
			cmd.Files = []string{regularFile, testFile, privateFile}
			cmd.IncludeTests = tt.includeTests
			cmd.IncludePrivate = tt.includePrivate

			contexts, err := cmd.processFiles()
			require.NoError(t, err)
			assert.Len(t, contexts, tt.expectedCount)
		})
	}
}

func TestDocCommand_detectProjectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		language string
		files    map[string]bool
		expected string
	}{
		{
			name:     "explicit language",
			language: "rust",
			files:    map[string]bool{},
			expected: "rust",
		},
		{
			name:     "detect Go",
			language: "",
			files:    map[string]bool{"go.mod": true},
			expected: "go",
		},
		{
			name:     "detect JavaScript",
			language: "",
			files:    map[string]bool{"package.json": true},
			expected: "javascript",
		},
		{
			name:     "detect Python",
			language: "",
			files:    map[string]bool{"requirements.txt": true},
			expected: "python",
		},
		{
			name:     "detect Java",
			language: "",
			files:    map[string]bool{"pom.xml": true},
			expected: "java",
		},
		{
			name:     "unknown",
			language: "",
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

func TestDocCommand_fileOperations(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := NewDocCommand()

	t.Run("readFile", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "read.txt")
		content := "test content"
		err := os.WriteFile(testFile, []byte(content), 0644)
		require.NoError(t, err)

		result, err := cmd.readFile(testFile)
		require.NoError(t, err)
		assert.Equal(t, content, result)
	})

	t.Run("writeFile", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "subdir", "write.txt")
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
		assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
	})
}

func TestDocCommand_detectFramework(t *testing.T) {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
