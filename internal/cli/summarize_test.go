package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dshills/sigil/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSummarizeCommand(t *testing.T) {
	cmd := NewSummarizeCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "summarize", cmd.Name)
	assert.Equal(t, "markdown", cmd.Format)
	assert.NotZero(t, cmd.startTime)
}

func TestSummarizeCommand_CreateCobraCommand(t *testing.T) {
	cmd := NewSummarizeCommand()
	cobraCmd := cmd.CreateCobraCommand()

	assert.NotNil(t, cobraCmd)
	assert.Equal(t, "summarize [files...]", cobraCmd.Use)
	assert.Contains(t, cobraCmd.Short, "Generate code summaries")

	// Check flags
	assert.NotNil(t, cobraCmd.Flags().Lookup("recursive"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("brief"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("focus"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("format"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("output"))
}

func TestSummarizeCommand_validateInputs(t *testing.T) {
	// Create temporary test files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0644))

	tests := []struct {
		name    string
		files   []string
		format  string
		wantErr bool
	}{
		{
			name:    "valid inputs",
			files:   []string{testFile},
			format:  "markdown",
			wantErr: false,
		},
		{
			name:    "no files",
			files:   []string{},
			format:  "markdown",
			wantErr: true,
		},
		{
			name:    "non-existent file",
			files:   []string{"/non/existent/file.go"},
			format:  "markdown",
			wantErr: true,
		},
		{
			name:    "invalid format",
			files:   []string{testFile},
			format:  "invalid",
			wantErr: true,
		},
		{
			name:    "text format",
			files:   []string{testFile},
			format:  "text",
			wantErr: false,
		},
		{
			name:    "json format",
			files:   []string{testFile},
			format:  "json",
			wantErr: false,
		},
		{
			name:    "html format",
			files:   []string{testFile},
			format:  "html",
			wantErr: false,
		},
		{
			name:    "yaml format",
			files:   []string{testFile},
			format:  "yaml",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewSummarizeCommand()
			cmd.Files = tt.files
			cmd.Format = tt.format

			err := cmd.validateInputs()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSummarizeCommand_buildDescription(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*SummarizeCommand)
		expected string
	}{
		{
			name:     "basic description",
			setup:    func(c *SummarizeCommand) {},
			expected: "Generate a comprehensive summary of the provided code",
		},
		{
			name: "with focus",
			setup: func(c *SummarizeCommand) {
				c.Focus = "architecture"
			},
			expected: "Generate a comprehensive summary of the provided code with focus on: architecture",
		},
		{
			name: "brief summary",
			setup: func(c *SummarizeCommand) {
				c.Brief = true
			},
			expected: "Generate a comprehensive summary of the provided code (brief summary requested)",
		},
		{
			name: "with focus and brief",
			setup: func(c *SummarizeCommand) {
				c.Focus = "dependencies"
				c.Brief = true
			},
			expected: "Generate a comprehensive summary of the provided code with focus on: dependencies (brief summary requested)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewSummarizeCommand()
			tt.setup(cmd)

			result := cmd.buildDescription()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSummarizeCommand_createSummarizeTask(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}"
	require.NoError(t, os.WriteFile(testFile, []byte(testContent), 0644))

	tests := []struct {
		name    string
		setup   func(*SummarizeCommand)
		wantErr bool
		check   func(*testing.T, *agent.Task)
	}{
		{
			name: "basic task creation",
			setup: func(c *SummarizeCommand) {
				c.Files = []string{testFile}
				c.Format = "markdown"
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				assert.Equal(t, agent.TaskTypeAnalyze, task.Type)
				assert.Equal(t, agent.PriorityMedium, task.Priority)
				assert.Len(t, task.Context.Files, 1)
				assert.Equal(t, testFile, task.Context.Files[0].Path)
				assert.Equal(t, testContent, task.Context.Files[0].Content)
				assert.Equal(t, "go", task.Context.Files[0].Language)
				assert.False(t, task.Context.Files[0].IsTarget)
				assert.True(t, task.Context.Files[0].IsReference)
			},
		},
		{
			name: "with focus",
			setup: func(c *SummarizeCommand) {
				c.Files = []string{testFile}
				c.Focus = "error handling"
				c.Format = "markdown"
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				found := false
				for _, req := range task.Context.Requirements {
					if strings.Contains(req, "Focus specifically on: error handling") {
						found = true
						break
					}
				}
				assert.True(t, found, "Focus requirement not found")
			},
		},
		{
			name: "brief summary",
			setup: func(c *SummarizeCommand) {
				c.Files = []string{testFile}
				c.Brief = true
				c.Format = "text"
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				found := false
				for _, req := range task.Context.Requirements {
					if strings.Contains(req, "concise, high-level summary") {
						found = true
						break
					}
				}
				assert.True(t, found, "Brief requirement not found")
			},
		},
		{
			name: "detailed summary",
			setup: func(c *SummarizeCommand) {
				c.Files = []string{testFile}
				c.Brief = false
				c.Format = "json"
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				detailReqs := []string{
					"implementation details",
					"code metrics",
				}
				for _, req := range detailReqs {
					found := false
					for _, taskReq := range task.Context.Requirements {
						if strings.Contains(taskReq, req) {
							found = true
							break
						}
					}
					assert.True(t, found, "Detailed requirement '%s' not found", req)
				}
			},
		},
		{
			name: "non-existent file",
			setup: func(c *SummarizeCommand) {
				c.Files = []string{"/non/existent/file.go"}
			},
			wantErr: true,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewSummarizeCommand()
			tt.setup(cmd)

			task, err := cmd.createSummarizeTask()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, task)
				if tt.check != nil {
					tt.check(t, task)
				}
			}
		})
	}
}

func TestSummarizeCommand_formatMarkdown(t *testing.T) {
	cmd := NewSummarizeCommand()
	cmd.Files = []string{"test.go", "main.go"}
	cmd.Focus = "architecture"

	content := "This is the summary content"
	result := cmd.formatMarkdown(content)

	assert.Contains(t, result, "# Code Summary")
	assert.Contains(t, result, "**Focus:** architecture")
	assert.Contains(t, result, "- `test.go`")
	assert.Contains(t, result, "- `main.go`")
	assert.Contains(t, result, "## Summary")
	assert.Contains(t, result, content)
}

func TestSummarizeCommand_formatText(t *testing.T) {
	cmd := NewSummarizeCommand()
	cmd.Files = []string{"test.go"}
	cmd.Focus = "dependencies"

	content := "Text summary"
	result := cmd.formatText(content)

	assert.Contains(t, result, "CODE SUMMARY")
	assert.Contains(t, result, "Focus: dependencies")
	assert.Contains(t, result, "- test.go")
	assert.Contains(t, result, "Summary:")
	assert.Contains(t, result, content)
}

func TestSummarizeCommand_formatJSON(t *testing.T) {
	cmd := NewSummarizeCommand()
	cmd.Files = []string{"file1.go", "file2.go"}
	cmd.Focus = "performance"
	cmd.Brief = true
	cmd.startTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	content := "JSON summary"
	result := cmd.formatJSON(content)

	assert.Contains(t, result, `"focus": "performance"`)
	assert.Contains(t, result, `"files": [`)
	assert.Contains(t, result, `"file1.go"`)
	assert.Contains(t, result, `"file2.go"`)
	assert.Contains(t, result, `"summary": "JSON summary"`)
	assert.Contains(t, result, `"format": "json"`)
	assert.Contains(t, result, `"brief": true`)
	assert.Contains(t, result, `"timestamp":`)
}

func TestSummarizeCommand_formatHTML(t *testing.T) {
	cmd := NewSummarizeCommand()
	cmd.Files = []string{"index.js"}
	cmd.Focus = "state management"

	content := "HTML content\nwith newlines"
	result := cmd.formatHTML(content)

	assert.Contains(t, result, "<!DOCTYPE html>")
	assert.Contains(t, result, "<title>Code Summary</title>")
	assert.Contains(t, result, "<strong>Focus:</strong> state management")
	assert.Contains(t, result, "<code>index.js</code>")
	assert.Contains(t, result, "HTML content<br>")
	assert.Contains(t, result, "with newlines")
}

func TestSummarizeCommand_formatYAML(t *testing.T) {
	cmd := NewSummarizeCommand()
	cmd.Files = []string{"app.py", "test.py"}
	cmd.Focus = "testing"
	cmd.Brief = false
	cmd.startTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	content := "Line 1\nLine 2\nLine 3"
	result := cmd.formatYAML(content)

	assert.Contains(t, result, "summary:")
	assert.Contains(t, result, `focus: "testing"`)
	assert.Contains(t, result, "files:")
	assert.Contains(t, result, `- "app.py"`)
	assert.Contains(t, result, `- "test.py"`)
	assert.Contains(t, result, "brief: false")
	assert.Contains(t, result, "timestamp:")
	assert.Contains(t, result, "content: |")
	assert.Contains(t, result, "    Line 1")
	assert.Contains(t, result, "    Line 2")
	assert.Contains(t, result, "    Line 3")
}

func TestSummarizeCommand_detectLanguage(t *testing.T) {
	tests := []struct {
		filePath string
		expected string
	}{
		{"test.go", "go"},
		{"main.js", "javascript"},
		{"app.ts", "javascript"},
		{"script.py", "python"},
		{"Main.java", "java"},
		{"program.cpp", "c++"},
		{"code.c", "c++"},
		{"lib.rs", "rust"},
		{"readme.txt", "text"},
		{"unknown", "text"},
	}

	cmd := NewSummarizeCommand()
	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			result := cmd.detectLanguage(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSummarizeCommand_detectProjectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected string
	}{
		{
			name:     "go project",
			files:    map[string]string{"go.mod": "module test"},
			expected: "go",
		},
		{
			name:     "go project with main",
			files:    map[string]string{"main.go": "package main"},
			expected: "go",
		},
		{
			name:     "javascript project",
			files:    map[string]string{"package.json": "{}"},
			expected: "javascript",
		},
		{
			name:     "python project",
			files:    map[string]string{"requirements.txt": "django"},
			expected: "python",
		},
		{
			name:     "python project with setup",
			files:    map[string]string{"setup.py": ""},
			expected: "python",
		},
		{
			name:     "java maven project",
			files:    map[string]string{"pom.xml": "<project>"},
			expected: "java",
		},
		{
			name:     "java gradle project",
			files:    map[string]string{"build.gradle": ""},
			expected: "java",
		},
		{
			name:     "unknown project",
			files:    map[string]string{"readme.txt": "hello"},
			expected: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			originalWd, _ := os.Getwd()
			defer os.Chdir(originalWd)

			// Create test files
			for name, content := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644))
			}

			require.NoError(t, os.Chdir(tmpDir))

			cmd := NewSummarizeCommand()
			result := cmd.detectProjectLanguage()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSummarizeCommand_detectFramework(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected string
	}{
		{
			name:     "next.js project",
			files:    map[string]string{"next.config.js": "module.exports = {}"},
			expected: "next.js",
		},
		{
			name:     "angular project",
			files:    map[string]string{"angular.json": "{}"},
			expected: "angular",
		},
		{
			name:     "vue project",
			files:    map[string]string{"vue.config.js": "module.exports = {}"},
			expected: "vue",
		},
		{
			name:     "no framework",
			files:    map[string]string{"index.js": "console.log('hello')"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			originalWd, _ := os.Getwd()
			defer os.Chdir(originalWd)

			// Create test files
			for name, content := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644))
			}

			require.NoError(t, os.Chdir(tmpDir))

			cmd := NewSummarizeCommand()
			result := cmd.detectFramework()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSummarizeCommand_fileOperations(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "test content"

	cmd := NewSummarizeCommand()

	// Test fileExists - non-existent
	assert.False(t, cmd.fileExists(testFile))

	// Test writeFile
	err := cmd.writeFile(testFile, testContent)
	assert.NoError(t, err)

	// Test fileExists - exists
	assert.True(t, cmd.fileExists(testFile))

	// Test readFile
	content, err := cmd.readFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, testContent, content)

	// Test readFile - non-existent
	_, err = cmd.readFile(filepath.Join(tmpDir, "nonexistent.txt"))
	assert.Error(t, err)
}

func TestSummarizeCommand_outputResult(t *testing.T) {
	tests := []struct {
		name         string
		result       *agent.OrchestrationResult
		outputFile   string
		format       string
		wantErr      bool
		checkContent func(t *testing.T, content string)
	}{
		{
			name: "successful output to stdout",
			result: &agent.OrchestrationResult{
				FinalResult: &agent.Result{
					Reasoning: "Test summary content",
				},
			},
			format:  "markdown",
			wantErr: false,
		},
		{
			name: "use artifact when reasoning empty",
			result: &agent.OrchestrationResult{
				FinalResult: &agent.Result{
					Reasoning: "",
					Artifacts: []agent.Artifact{
						{Content: "Artifact summary"},
					},
				},
			},
			format:  "text",
			wantErr: false,
		},
		{
			name: "no final result",
			result: &agent.OrchestrationResult{
				FinalResult: nil,
			},
			wantErr: true,
		},
		{
			name: "no content available",
			result: &agent.OrchestrationResult{
				FinalResult: &agent.Result{
					Reasoning: "",
					Artifacts: []agent.Artifact{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewSummarizeCommand()
			cmd.Format = tt.format
			cmd.OutputFile = tt.outputFile

			err := cmd.outputResult(tt.result)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSummarizeCommand_formatOutput(t *testing.T) {
	tests := []struct {
		format   string
		content  string
		wantErr  bool
		contains []string
	}{
		{
			format:   "markdown",
			content:  "test content",
			wantErr:  false,
			contains: []string{"# Code Summary", "test content"},
		},
		{
			format:   "text",
			content:  "test content",
			wantErr:  false,
			contains: []string{"CODE SUMMARY", "test content"},
		},
		{
			format:   "json",
			content:  "test content",
			wantErr:  false,
			contains: []string{`"summary": "test content"`},
		},
		{
			format:   "html",
			content:  "test content",
			wantErr:  false,
			contains: []string{"<title>Code Summary</title>", "test content"},
		},
		{
			format:   "yaml",
			content:  "test content",
			wantErr:  false,
			contains: []string{"summary:", "content: |", "test content"},
		},
		{
			format:   "unknown",
			content:  "test content",
			wantErr:  false,
			contains: []string{"test content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			cmd := NewSummarizeCommand()
			cmd.Format = tt.format
			cmd.Files = []string{"test.go"}

			result, err := cmd.formatOutput(tt.content)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for _, expected := range tt.contains {
					assert.Contains(t, result, expected)
				}
			}
		})
	}
}

func TestSummarizeCommand_formatYAMLIndentation(t *testing.T) {
	cmd := NewSummarizeCommand()
	cmd.Files = []string{"test.go"}
	cmd.Focus = ""

	// Test multi-line content indentation
	content := "First line\nSecond line\n  Indented line\nLast line"
	result := cmd.formatYAML(content)

	// Check that each content line is properly indented
	lines := strings.Split(result, "\n")
	foundContent := false
	for i, line := range lines {
		if strings.Contains(line, "content: |") {
			foundContent = true
			// Check subsequent lines are indented
			assert.Contains(t, lines[i+1], "    First line")
			assert.Contains(t, lines[i+2], "    Second line")
			assert.Contains(t, lines[i+3], "      Indented line") // Original indentation preserved
			assert.Contains(t, lines[i+4], "    Last line")
			break
		}
	}
	assert.True(t, foundContent, "content: | line not found")
}
