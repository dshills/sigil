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

func TestNewExplainCommand(t *testing.T) {
	cmd := NewExplainCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "explain", cmd.BaseCommand.Name)
	assert.Equal(t, "markdown", cmd.Format)
	assert.NotZero(t, cmd.startTime)
}

func TestExplainCommand_CreateCobraCommand(t *testing.T) {
	cmd := NewExplainCommand()
	cobraCmd := cmd.CreateCobraCommand()
	
	assert.NotNil(t, cobraCmd)
	assert.Equal(t, "explain [files...]", cobraCmd.Use)
	assert.Contains(t, cobraCmd.Short, "Explain code files")
	
	// Check flags
	assert.NotNil(t, cobraCmd.Flags().Lookup("query"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("detailed"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("format"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("output"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("interactive"))
}

func TestExplainCommand_validateInputs(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewExplainCommand()
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

func TestExplainCommand_buildDescription(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ExplainCommand)
		expected string
	}{
		{
			name:     "basic description",
			setup:    func(c *ExplainCommand) {},
			expected: "Explain and analyze the provided code files",
		},
		{
			name: "with query",
			setup: func(c *ExplainCommand) {
				c.Query = "error handling"
			},
			expected: "Explain and analyze the provided code files with focus on: error handling",
		},
		{
			name: "detailed analysis",
			setup: func(c *ExplainCommand) {
				c.Detailed = true
			},
			expected: "Explain and analyze the provided code files (detailed analysis requested)",
		},
		{
			name: "with query and detailed",
			setup: func(c *ExplainCommand) {
				c.Query = "design patterns"
				c.Detailed = true
			},
			expected: "Explain and analyze the provided code files with focus on: design patterns (detailed analysis requested)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewExplainCommand()
			tt.setup(cmd)
			
			result := cmd.buildDescription()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExplainCommand_createExplainTask(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}"
	require.NoError(t, os.WriteFile(testFile, []byte(testContent), 0644))

	tests := []struct {
		name    string
		setup   func(*ExplainCommand)
		wantErr bool
		check   func(*testing.T, *agent.Task)
	}{
		{
			name: "basic task creation",
			setup: func(c *ExplainCommand) {
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
			name: "with query",
			setup: func(c *ExplainCommand) {
				c.Files = []string{testFile}
				c.Query = "function usage"
				c.Format = "markdown"
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				found := false
				for _, req := range task.Context.Requirements {
					if strings.Contains(req, "Focus on: function usage") {
						found = true
						break
					}
				}
				assert.True(t, found, "Query requirement not found")
			},
		},
		{
			name: "detailed analysis",
			setup: func(c *ExplainCommand) {
				c.Files = []string{testFile}
				c.Detailed = true
				c.Format = "json"
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				detailReqs := []string{
					"line-by-line analysis",
					"complex algorithms",
					"examples and use cases",
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
			setup: func(c *ExplainCommand) {
				c.Files = []string{"/non/existent/file.go"}
			},
			wantErr: true,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewExplainCommand()
			tt.setup(cmd)
			
			task, err := cmd.createExplainTask()
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

func TestExplainCommand_formatMarkdown(t *testing.T) {
	cmd := NewExplainCommand()
	cmd.Files = []string{"test.go", "main.go"}
	cmd.Query = "error handling"
	
	content := "This is the explanation content"
	result := cmd.formatMarkdown(content)
	
	assert.Contains(t, result, "# Code Explanation")
	assert.Contains(t, result, "**Query:** error handling")
	assert.Contains(t, result, "- `test.go`")
	assert.Contains(t, result, "- `main.go`")
	assert.Contains(t, result, "## Explanation")
	assert.Contains(t, result, content)
}

func TestExplainCommand_formatText(t *testing.T) {
	cmd := NewExplainCommand()
	cmd.Files = []string{"test.go"}
	cmd.Query = "patterns"
	
	content := "Text explanation"
	result := cmd.formatText(content)
	
	assert.Contains(t, result, "CODE EXPLANATION")
	assert.Contains(t, result, "Query: patterns")
	assert.Contains(t, result, "- test.go")
	assert.Contains(t, result, "Explanation:")
	assert.Contains(t, result, content)
}

func TestExplainCommand_formatJSON(t *testing.T) {
	cmd := NewExplainCommand()
	cmd.Files = []string{"file1.go", "file2.go"}
	cmd.Query = "test query"
	cmd.startTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	
	content := "JSON explanation"
	result := cmd.formatJSON(content)
	
	assert.Contains(t, result, `"query": "test query"`)
	assert.Contains(t, result, `"files": [`)
	assert.Contains(t, result, `"file1.go"`)
	assert.Contains(t, result, `"file2.go"`)
	assert.Contains(t, result, `"explanation": "JSON explanation"`)
	assert.Contains(t, result, `"format": "json"`)
	assert.Contains(t, result, `"timestamp":`)
}

func TestExplainCommand_formatHTML(t *testing.T) {
	cmd := NewExplainCommand()
	cmd.Files = []string{"index.js"}
	cmd.Query = "async operations"
	
	content := "HTML content\nwith newlines"
	result := cmd.formatHTML(content)
	
	assert.Contains(t, result, "<!DOCTYPE html>")
	assert.Contains(t, result, "<title>Code Explanation</title>")
	assert.Contains(t, result, "<strong>Query:</strong> async operations")
	assert.Contains(t, result, "<code>index.js</code>")
	assert.Contains(t, result, "HTML content<br>")
	assert.Contains(t, result, "with newlines")
}

func TestExplainCommand_detectLanguage(t *testing.T) {
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

	cmd := NewExplainCommand()
	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			result := cmd.detectLanguage(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExplainCommand_detectProjectLanguage(t *testing.T) {
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
			
			cmd := NewExplainCommand()
			result := cmd.detectProjectLanguage()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExplainCommand_detectFramework(t *testing.T) {
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
			
			cmd := NewExplainCommand()
			result := cmd.detectFramework()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExplainCommand_fileOperations(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "test content"
	
	cmd := NewExplainCommand()
	
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

func TestExplainCommand_outputResult(t *testing.T) {
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
					Reasoning: "Test explanation content",
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
						{Content: "Artifact explanation"},
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
			cmd := NewExplainCommand()
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

func TestExplainCommand_formatOutput(t *testing.T) {
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
			contains: []string{"# Code Explanation", "test content"},
		},
		{
			format:   "text",
			content:  "test content",
			wantErr:  false,
			contains: []string{"CODE EXPLANATION", "test content"},
		},
		{
			format:   "json",
			content:  "test content",
			wantErr:  false,
			contains: []string{`"explanation": "test content"`},
		},
		{
			format:   "html",
			content:  "test content",
			wantErr:  false,
			contains: []string{"<title>Code Explanation</title>", "test content"},
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
			cmd := NewExplainCommand()
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