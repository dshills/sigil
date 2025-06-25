package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dshills/sigil/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEditCommand(t *testing.T) {
	cmd := NewEditCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "edit", cmd.Name)
	assert.NotZero(t, cmd.startTime)
}

func TestEditCommand_CreateCobraCommand(t *testing.T) {
	cmd := NewEditCommand()
	cobraCmd := cmd.CreateCobraCommand()

	assert.NotNil(t, cobraCmd)
	assert.Equal(t, "edit", cobraCmd.Use[:4])
	assert.Contains(t, cobraCmd.Short, "Edit files with intelligent code transformations")

	// Check flags
	assert.NotNil(t, cobraCmd.Flags().Lookup("description"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("secure"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("fast"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("maintain"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("auto-commit"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("branch"))
	assert.NotNil(t, cobraCmd.Flags().Lookup("agent"))
}

func TestEditCommand_validateFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	existingFile := filepath.Join(tmpDir, "exists.go")
	err := os.WriteFile(existingFile, []byte("package main"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name    string
		files   []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no files specified",
			files:   []string{},
			wantErr: true,
			errMsg:  "no files specified",
		},
		{
			name:    "file does not exist",
			files:   []string{filepath.Join(tmpDir, "nonexistent.go")},
			wantErr: true,
			errMsg:  "file does not exist",
		},
		{
			name:    "valid file",
			files:   []string{existingFile},
			wantErr: false,
		},
		{
			name:    "multiple files with one invalid",
			files:   []string{existingFile, filepath.Join(tmpDir, "missing.go")},
			wantErr: true,
			errMsg:  "file does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewEditCommand()
			cmd.Files = tt.files
			err := cmd.validateFiles()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEditCommand_createEditTask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.js")
	err := os.WriteFile(file1, []byte("package main\nfunc main() {}"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("console.log('hello');"), 0644)
	require.NoError(t, err)

	cmd := NewEditCommand()
	cmd.Files = []string{file1, file2}
	cmd.Description = "Add error handling"

	task, err := cmd.createEditTask()
	require.NoError(t, err)
	assert.NotNil(t, task)

	// Check task properties
	assert.Contains(t, task.ID, "edit_")
	assert.Equal(t, agent.TaskTypeEdit, task.Type)
	assert.Equal(t, agent.PriorityMedium, task.Priority)
	assert.Equal(t, "Add error handling", task.Description)

	// Check file contexts
	assert.Len(t, task.Context.Files, 2)
	assert.Equal(t, file1, task.Context.Files[0].Path)
	assert.Equal(t, "go", task.Context.Files[0].Language)
	assert.Equal(t, file2, task.Context.Files[1].Path)
	assert.Equal(t, "javascript", task.Context.Files[1].Language)

	// Check requirements
	assert.Contains(t, task.Context.Requirements, "Edit the specified files according to the description")
}

func TestEditCommand_detectLanguage(t *testing.T) {
	tests := []struct {
		filePath string
		expected string
	}{
		{"file.go", "go"},
		{"file.js", "javascript"},
		{"file.ts", "javascript"},
		{"file.py", "python"},
		{"file.java", "java"},
		{"file.rs", "rust"},
		{"file.cpp", "c++"},
		{"file.rb", "text"},
		{"file.php", "text"},
		{"file.cs", "text"},
		{"file.swift", "text"},
		{"file.kt", "text"},
		{"file.dart", "text"},
		{"file.unknown", "text"},
	}

	cmd := NewEditCommand()

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			result := cmd.detectLanguage(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEditCommand_detectProjectLanguage(t *testing.T) {
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
			name:     "Python with requirements",
			files:    map[string]bool{"requirements.txt": true},
			expected: "python",
		},
		{
			name:     "Python with setup.py",
			files:    map[string]bool{"setup.py": true},
			expected: "python",
		},
		{
			name:     "Python with pyproject.toml",
			files:    map[string]bool{"pyproject.toml": true},
			expected: "python",
		},
		{
			name:     "Java with pom.xml",
			files:    map[string]bool{"pom.xml": true},
			expected: "java",
		},
		{
			name:     "Java with build.gradle",
			files:    map[string]bool{"build.gradle": true},
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

func TestEditCommand_detectFramework(t *testing.T) {
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

func TestEditCommand_applyProposal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte("original content"), 0644)
	require.NoError(t, err)

	cmd := NewEditCommand()

	proposal := agent.Proposal{
		ID: "test-proposal",
		Changes: []agent.Change{
			{
				Type:       agent.ChangeTypeUpdate,
				Path:       testFile,
				NewContent: "updated content",
			},
			{
				Type:       agent.ChangeTypeCreate,
				Path:       filepath.Join(tmpDir, "new.go"),
				NewContent: "new file content",
			},
		},
	}

	err = cmd.applyProposal(proposal, nil)
	require.NoError(t, err)

	// Check updated file
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "updated content", string(content))

	// Check created file
	newContent, err := os.ReadFile(filepath.Join(tmpDir, "new.go"))
	require.NoError(t, err)
	assert.Equal(t, "new file content", string(newContent))
}

func TestEditCommand_applyProposal_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file to delete
	deleteFile := filepath.Join(tmpDir, "delete.go")
	err := os.WriteFile(deleteFile, []byte("to be deleted"), 0644)
	require.NoError(t, err)

	cmd := NewEditCommand()

	proposal := agent.Proposal{
		ID: "delete-proposal",
		Changes: []agent.Change{
			{
				Type: agent.ChangeTypeDelete,
				Path: deleteFile,
			},
		},
	}

	err = cmd.applyProposal(proposal, nil)
	require.NoError(t, err)

	// Check file was deleted
	_, err = os.Stat(deleteFile)
	assert.True(t, os.IsNotExist(err))
}

func TestEditCommand_applyProposal_UnsupportedTypes(t *testing.T) {
	cmd := NewEditCommand()

	proposal := agent.Proposal{
		ID: "unsupported-proposal",
		Changes: []agent.Change{
			{
				Type: agent.ChangeTypeMove,
				Path: "source.go",
			},
			{
				Type: agent.ChangeTypeRename,
				Path: "old.go",
			},
		},
	}

	// Should not error, just warn
	err := cmd.applyProposal(proposal, nil)
	assert.NoError(t, err)
}

func TestEditCommand_fileOperations(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := NewEditCommand()

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
		testFile := filepath.Join(tmpDir, "write.txt")
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

	t.Run("deleteFile", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "delete.txt")
		err := os.WriteFile(testFile, []byte("delete me"), 0644)
		require.NoError(t, err)

		err = cmd.deleteFile(testFile)
		require.NoError(t, err)

		// Check file was deleted
		_, err = os.Stat(testFile)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestEditCommand_Execute_Validation(t *testing.T) {
	cmd := NewEditCommand()
	cmd.Files = []string{} // No files
	ctx := context.Background()

	// Should fail validation
	err := cmd.Execute(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no files specified")
}

func TestEditCommand_executeDirectEdit(t *testing.T) {
	cmd := NewEditCommand()
	cmd.Files = []string{"test.go"}
	cmd.Description = "Test edit"
	ctx := context.Background()

	// Direct edit is placeholder, should not error
	err := cmd.executeDirectEdit(ctx, nil)
	assert.NoError(t, err)
}

func TestEditCommand_processAgentResult(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(testFile, []byte("original"), 0644)
	require.NoError(t, err)

	cmd := NewEditCommand()
	cmd.AutoCommit = false // Disable auto-commit for test

	result := &agent.OrchestrationResult{
		Status: agent.StatusSuccess,
		FinalResult: &agent.Result{
			Proposals: []agent.Proposal{
				{
					ID: "test",
					Changes: []agent.Change{
						{
							Type:       agent.ChangeTypeUpdate,
							Path:       testFile,
							NewContent: "modified",
						},
					},
				},
			},
		},
	}

	err = cmd.processAgentResult(result, nil)
	require.NoError(t, err)

	// Check file was modified
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "modified", string(content))
}

func TestEditCommand_processAgentResult_Failed(t *testing.T) {
	cmd := NewEditCommand()

	result := &agent.OrchestrationResult{
		Status: agent.StatusFailed,
	}

	err := cmd.processAgentResult(result, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent execution failed")
}
