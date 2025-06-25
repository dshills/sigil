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

func TestNewReviewCommand(t *testing.T) {
	cmd := NewReviewCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "review", cmd.BaseCommand.Name)
	assert.Equal(t, "warning", cmd.Severity)
	assert.Equal(t, "markdown", cmd.Format)
	assert.NotZero(t, cmd.startTime)
}

func TestReviewCommand_CreateCobraCommand(t *testing.T) {
	cmd := NewReviewCommand()
	cobraCmd := cmd.CreateCobraCommand()
	
	assert.NotNil(t, cobraCmd)
	assert.Equal(t, "review [files...]", cobraCmd.Use)
	assert.Contains(t, cobraCmd.Short, "Review code")
	
	// Check flags
	assert.NotNil(t, cobraCmd.Flags().Lookup("focus"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("severity"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("format"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("output"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("include-tests"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("check-security"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("check-performance"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("check-style"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("auto-fix"))
}

func TestReviewCommand_validateInputs(t *testing.T) {
	// Create temporary test files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0644))

	tests := []struct {
		name     string
		files    []string
		severity string
		format   string
		wantErr  bool
	}{
		{
			name:     "valid inputs",
			files:    []string{testFile},
			severity: "warning",
			format:   "markdown",
			wantErr:  false,
		},
		{
			name:     "no files",
			files:    []string{},
			severity: "warning",
			format:   "markdown",
			wantErr:  true,
		},
		{
			name:     "non-existent file",
			files:    []string{"/non/existent/file.go"},
			severity: "warning",
			format:   "markdown",
			wantErr:  true,
		},
		{
			name:     "invalid severity",
			files:    []string{testFile},
			severity: "invalid",
			format:   "markdown",
			wantErr:  true,
		},
		{
			name:     "error severity",
			files:    []string{testFile},
			severity: "error",
			format:   "markdown",
			wantErr:  false,
		},
		{
			name:     "info severity",
			files:    []string{testFile},
			severity: "info",
			format:   "markdown",
			wantErr:  false,
		},
		{
			name:     "all severity",
			files:    []string{testFile},
			severity: "all",
			format:   "markdown",
			wantErr:  false,
		},
		{
			name:     "invalid format",
			files:    []string{testFile},
			severity: "warning",
			format:   "invalid",
			wantErr:  true,
		},
		{
			name:     "text format",
			files:    []string{testFile},
			severity: "warning",
			format:   "text",
			wantErr:  false,
		},
		{
			name:     "json format",
			files:    []string{testFile},
			severity: "warning",
			format:   "json",
			wantErr:  false,
		},
		{
			name:     "xml format",
			files:    []string{testFile},
			severity: "warning",
			format:   "xml",
			wantErr:  false,
		},
		{
			name:     "sarif format",
			files:    []string{testFile},
			severity: "warning",
			format:   "sarif",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewReviewCommand()
			cmd.Files = tt.files
			cmd.Severity = tt.severity
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

func TestReviewCommand_buildDescription(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ReviewCommand)
		expected string
	}{
		{
			name:     "basic description",
			setup:    func(c *ReviewCommand) {},
			expected: "Perform comprehensive code review of the provided files",
		},
		{
			name: "with focus areas",
			setup: func(c *ReviewCommand) {
				c.Focus = []string{"security", "performance"}
			},
			expected: "Perform comprehensive code review of the provided files with focus on: security, performance",
		},
		{
			name: "with auto-fix",
			setup: func(c *ReviewCommand) {
				c.AutoFix = true
			},
			expected: "Perform comprehensive code review of the provided files (auto-fix enabled)",
		},
		{
			name: "with focus and auto-fix",
			setup: func(c *ReviewCommand) {
				c.Focus = []string{"testing"}
				c.AutoFix = true
			},
			expected: "Perform comprehensive code review of the provided files with focus on: testing (auto-fix enabled)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewReviewCommand()
			tt.setup(cmd)
			
			result := cmd.buildDescription()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReviewCommand_createReviewTask(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}"
	require.NoError(t, os.WriteFile(testFile, []byte(testContent), 0644))

	tests := []struct {
		name    string
		setup   func(*ReviewCommand)
		wantErr bool
		check   func(*testing.T, *agent.Task)
	}{
		{
			name: "basic task creation",
			setup: func(c *ReviewCommand) {
				c.Files = []string{testFile}
				c.Severity = "warning"
				c.Format = "markdown"
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				assert.Equal(t, agent.TaskTypeReview, task.Type)
				assert.Equal(t, agent.PriorityHigh, task.Priority)
				assert.Len(t, task.Context.Files, 1)
				assert.Equal(t, testFile, task.Context.Files[0].Path)
				assert.Equal(t, testContent, task.Context.Files[0].Content)
				assert.Equal(t, "go", task.Context.Files[0].Language)
				assert.True(t, task.Context.Files[0].IsTarget)
				assert.False(t, task.Context.Files[0].IsReference)
			},
		},
		{
			name: "with focus areas",
			setup: func(c *ReviewCommand) {
				c.Files = []string{testFile}
				c.Focus = []string{"security", "performance"}
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				found := false
				for _, req := range task.Context.Requirements {
					if strings.Contains(req, "Focus specifically on: security, performance") {
						found = true
						break
					}
				}
				assert.True(t, found, "Focus requirement not found")
			},
		},
		{
			name: "with include tests",
			setup: func(c *ReviewCommand) {
				c.Files = []string{testFile}
				c.IncludeTests = true
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				found := false
				for _, req := range task.Context.Requirements {
					if strings.Contains(req, "test coverage") {
						found = true
						break
					}
				}
				assert.True(t, found, "Test coverage requirement not found")
			},
		},
		{
			name: "with security check",
			setup: func(c *ReviewCommand) {
				c.Files = []string{testFile}
				c.CheckSecurity = true
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				found := false
				for _, constraint := range task.Constraints {
					if constraint.Type == agent.ConstraintTypeSecurity {
						found = true
						assert.Equal(t, agent.SeverityError, constraint.Severity)
						break
					}
				}
				assert.True(t, found, "Security constraint not found")
			},
		},
		{
			name: "with performance check",
			setup: func(c *ReviewCommand) {
				c.Files = []string{testFile}
				c.CheckPerformance = true
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				found := false
				for _, constraint := range task.Constraints {
					if constraint.Type == agent.ConstraintTypePerformance {
						found = true
						assert.Equal(t, agent.SeverityWarning, constraint.Severity)
						break
					}
				}
				assert.True(t, found, "Performance constraint not found")
			},
		},
		{
			name: "with style check",
			setup: func(c *ReviewCommand) {
				c.Files = []string{testFile}
				c.CheckStyle = true
			},
			wantErr: false,
			check: func(t *testing.T, task *agent.Task) {
				found := false
				for _, constraint := range task.Constraints {
					if constraint.Type == agent.ConstraintTypeStyle {
						found = true
						assert.Equal(t, agent.SeverityInfo, constraint.Severity)
						break
					}
				}
				assert.True(t, found, "Style constraint not found")
			},
		},
		{
			name: "non-existent file",
			setup: func(c *ReviewCommand) {
				c.Files = []string{"/non/existent/file.go"}
			},
			wantErr: true,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewReviewCommand()
			tt.setup(cmd)
			
			task, err := cmd.createReviewTask()
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

func TestReviewCommand_formatMarkdown(t *testing.T) {
	cmd := NewReviewCommand()
	cmd.Files = []string{"test.go", "main.go"}
	cmd.Focus = []string{"security", "performance"}
	cmd.Severity = "error"
	cmd.startTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	
	content := "Review findings content"
	result := &agent.OrchestrationResult{
		Status: agent.StatusSuccess,
		Results: []agent.Result{
			{}, {}, {}, // 3 findings
		},
	}
	
	formatted := cmd.formatMarkdown(content, result)
	
	assert.Contains(t, formatted, "# Code Review Report")
	assert.Contains(t, formatted, "**Focus Areas:** security, performance")
	assert.Contains(t, formatted, "- `test.go`")
	assert.Contains(t, formatted, "- `main.go`")
	assert.Contains(t, formatted, "**Severity Filter:** error")
	assert.Contains(t, formatted, "**Review Status:** success")
	assert.Contains(t, formatted, "**Total Findings:** 3")
	assert.Contains(t, formatted, "## Review Details")
	assert.Contains(t, formatted, content)
}

func TestReviewCommand_formatText(t *testing.T) {
	cmd := NewReviewCommand()
	cmd.Files = []string{"test.go"}
	cmd.Focus = []string{"testing"}
	cmd.Severity = "warning"
	
	content := "Text review"
	result := &agent.OrchestrationResult{
		Status: agent.StatusSuccess,
		Results: []agent.Result{{}},
	}
	
	formatted := cmd.formatText(content, result)
	
	assert.Contains(t, formatted, "CODE REVIEW REPORT")
	assert.Contains(t, formatted, "Focus Areas: testing")
	assert.Contains(t, formatted, "- test.go")
	assert.Contains(t, formatted, "Severity Filter: warning")
	assert.Contains(t, formatted, "Review Status: success")
	assert.Contains(t, formatted, "Total Findings: 1")
	assert.Contains(t, formatted, content)
}

func TestReviewCommand_formatJSON(t *testing.T) {
	cmd := NewReviewCommand()
	cmd.Files = []string{"file1.go", "file2.go"}
	cmd.Focus = []string{"security"}
	cmd.Severity = "error"
	cmd.startTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	
	content := "JSON review"
	result := &agent.OrchestrationResult{
		Status: agent.StatusSuccess,
		Results: []agent.Result{{}, {}},
	}
	
	formatted := cmd.formatJSON(content, result)
	
	assert.Contains(t, formatted, `"focus_areas": [`)
	assert.Contains(t, formatted, `"security"`)
	assert.Contains(t, formatted, `"files": [`)
	assert.Contains(t, formatted, `"file1.go"`)
	assert.Contains(t, formatted, `"file2.go"`)
	assert.Contains(t, formatted, `"severity": "error"`)
	assert.Contains(t, formatted, `"status": "success"`)
	assert.Contains(t, formatted, `"findings_count": 2`)
	assert.Contains(t, formatted, `"content": "JSON review"`)
}

func TestReviewCommand_formatXML(t *testing.T) {
	cmd := NewReviewCommand()
	cmd.Files = []string{"app.js"}
	cmd.Focus = []string{"style", "performance"}
	cmd.Severity = "info"
	cmd.startTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	
	content := "XML review content"
	result := &agent.OrchestrationResult{
		Status: agent.StatusSuccess,
		Results: []agent.Result{},
	}
	
	formatted := cmd.formatXML(content, result)
	
	assert.Contains(t, formatted, `<?xml version="1.0" encoding="UTF-8"?>`)
	assert.Contains(t, formatted, "<review>")
	assert.Contains(t, formatted, "<severity>info</severity>")
	assert.Contains(t, formatted, "<status>success</status>")
	assert.Contains(t, formatted, "<findings_count>0</findings_count>")
	assert.Contains(t, formatted, "<area>style</area>")
	assert.Contains(t, formatted, "<area>performance</area>")
	assert.Contains(t, formatted, "<file>app.js</file>")
	assert.Contains(t, formatted, "<![CDATA[")
	assert.Contains(t, formatted, content)
	assert.Contains(t, formatted, "]]></content>")
	assert.Contains(t, formatted, "</review>")
}

func TestReviewCommand_formatSARIF(t *testing.T) {
	cmd := NewReviewCommand()
	cmd.Files = []string{"main.go"}
	cmd.Severity = "error"
	
	content := "SARIF review"
	result := &agent.OrchestrationResult{
		Status: agent.StatusSuccess,
		Results: []agent.Result{{}, {}, {}, {}, {}}, // 5 findings
	}
	
	formatted := cmd.formatSARIF(content, result)
	
	assert.Contains(t, formatted, `"$schema": "https://schemastore.azurewebsites.net/schemas/json/sarif-2.1.0.json"`)
	assert.Contains(t, formatted, `"version": "2.1.0"`)
	assert.Contains(t, formatted, `"name": "Sigil Code Review"`)
	assert.Contains(t, formatted, `"ruleId": "comprehensive-review"`)
	assert.Contains(t, formatted, `"level": "error"`)
	assert.Contains(t, formatted, `"text": "Code review completed with 5 findings"`)
	assert.Contains(t, formatted, `"uri": "main.go"`)
}

func TestReviewCommand_detectLanguage(t *testing.T) {
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

	cmd := NewReviewCommand()
	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			result := cmd.detectLanguage(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReviewCommand_detectProjectLanguage(t *testing.T) {
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
			name:     "java project",
			files:    map[string]string{"pom.xml": "<project>"},
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
			
			cmd := NewReviewCommand()
			result := cmd.detectProjectLanguage()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReviewCommand_detectFramework(t *testing.T) {
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
			
			cmd := NewReviewCommand()
			result := cmd.detectFramework()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReviewCommand_fileOperations(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "test content"
	
	cmd := NewReviewCommand()
	
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
	
	// Test deleteFile
	err = cmd.deleteFile(testFile)
	assert.NoError(t, err)
	assert.False(t, cmd.fileExists(testFile))
}

func TestReviewCommand_outputResult(t *testing.T) {
	tests := []struct {
		name       string
		result     *agent.OrchestrationResult
		outputFile string
		format     string
		wantErr    bool
	}{
		{
			name: "successful output to stdout",
			result: &agent.OrchestrationResult{
				FinalResult: &agent.Result{
					Reasoning: "Test review content",
				},
				Status: agent.StatusSuccess,
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
						{Content: "Artifact review"},
					},
				},
				Status: agent.StatusSuccess,
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
			cmd := NewReviewCommand()
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

func TestReviewCommand_applyProposal(t *testing.T) {
	tmpDir := t.TempDir()
	
	tests := []struct {
		name     string
		proposal agent.Proposal
		setup    func()
		wantErr  bool
		check    func()
	}{
		{
			name: "update file",
			proposal: agent.Proposal{
				ID: "test-update",
				Changes: []agent.Change{
					{
						Type:       agent.ChangeTypeUpdate,
						Path:       filepath.Join(tmpDir, "update.txt"),
						NewContent: "updated content",
					},
				},
			},
			setup: func() {
				os.WriteFile(filepath.Join(tmpDir, "update.txt"), []byte("old content"), 0644)
			},
			wantErr: false,
			check: func() {
				content, _ := os.ReadFile(filepath.Join(tmpDir, "update.txt"))
				assert.Equal(t, "updated content", string(content))
			},
		},
		{
			name: "create file",
			proposal: agent.Proposal{
				ID: "test-create",
				Changes: []agent.Change{
					{
						Type:       agent.ChangeTypeCreate,
						Path:       filepath.Join(tmpDir, "new.txt"),
						NewContent: "new content",
					},
				},
			},
			setup:   func() {},
			wantErr: false,
			check: func() {
				content, _ := os.ReadFile(filepath.Join(tmpDir, "new.txt"))
				assert.Equal(t, "new content", string(content))
			},
		},
		{
			name: "delete file",
			proposal: agent.Proposal{
				ID: "test-delete",
				Changes: []agent.Change{
					{
						Type: agent.ChangeTypeDelete,
						Path: filepath.Join(tmpDir, "delete.txt"),
					},
				},
			},
			setup: func() {
				os.WriteFile(filepath.Join(tmpDir, "delete.txt"), []byte("to delete"), 0644)
			},
			wantErr: false,
			check: func() {
				_, err := os.Stat(filepath.Join(tmpDir, "delete.txt"))
				assert.True(t, os.IsNotExist(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			
			cmd := NewReviewCommand()
			err := cmd.applyProposal(tt.proposal)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.check()
			}
		})
	}
}

func TestReviewCommand_formatOutput(t *testing.T) {
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
			contains: []string{"# Code Review Report", "test content"},
		},
		{
			format:   "text",
			content:  "test content",
			wantErr:  false,
			contains: []string{"CODE REVIEW REPORT", "test content"},
		},
		{
			format:   "json",
			content:  "test content",
			wantErr:  false,
			contains: []string{`"content": "test content"`},
		},
		{
			format:   "xml",
			content:  "test content",
			wantErr:  false,
			contains: []string{"<review>", "test content"},
		},
		{
			format:   "sarif",
			content:  "test content",
			wantErr:  false,
			contains: []string{"sarif-2.1.0", "Sigil Code Review"},
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
			cmd := NewReviewCommand()
			cmd.Format = tt.format
			cmd.Files = []string{"test.go"}
			
			result := &agent.OrchestrationResult{
				Status: agent.StatusSuccess,
				Results: []agent.Result{},
			}
			
			formatted, err := cmd.formatOutput(tt.content, result)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for _, expected := range tt.contains {
					assert.Contains(t, formatted, expected)
				}
			}
		})
	}
}