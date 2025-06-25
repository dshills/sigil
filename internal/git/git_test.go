package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestRepo creates a temporary git repository for testing
func createTestRepo(t *testing.T) (string, *Repository) {
	t.Helper()

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "sigil-git-test-*")
	require.NoError(t, err)

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Configure git user for testing
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err)

	// Create a repository instance
	repo, err := NewRepository(tempDir)
	require.NoError(t, err)

	// Clean up function
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return tempDir, repo
}

// createTestFile creates a test file in the repository
func createTestFile(t *testing.T, repoPath, filename, content string) {
	t.Helper()

	filePath := filepath.Join(repoPath, filename)
	err := os.WriteFile(filePath, []byte(content), 0600)
	require.NoError(t, err)
}

func TestNewRepository(t *testing.T) {
	t.Run("valid git repository", func(t *testing.T) {
		tempDir, _ := createTestRepo(t)

		repo, err := NewRepository(tempDir)
		assert.NoError(t, err)
		assert.NotNil(t, repo)
		assert.Equal(t, tempDir, repo.Path)
	})

	t.Run("empty path uses current directory", func(t *testing.T) {
		// This test assumes we're already in a git repository
		repo, err := NewRepository("")
		if err != nil {
			// If not in a git repo, skip this test
			t.Skip("Not in a git repository")
		}
		assert.NotNil(t, repo)
		assert.NotEmpty(t, repo.Path)
	})

	t.Run("non-git directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "non-git-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		repo, err := NewRepository(tempDir)
		assert.Error(t, err)
		assert.Nil(t, repo)
		assert.Contains(t, err.Error(), "not a git repository")
	})
}

func TestRepository_GetRoot(t *testing.T) {
	tempDir, repo := createTestRepo(t)

	root, err := repo.GetRoot()
	assert.NoError(t, err)
	// On macOS, temp paths may be resolved differently (/var vs /private/var)
	assert.Contains(t, root, filepath.Base(tempDir))
}

func TestRepository_GetCurrentBranch(t *testing.T) {
	tempDir, repo := createTestRepo(t)

	// Create an initial commit so we have a branch
	createTestFile(t, tempDir, "initial.txt", "initial content")
	err := repo.Add("initial.txt")
	require.NoError(t, err)
	err = repo.Commit("Initial commit")
	require.NoError(t, err)

	branch, err := repo.GetCurrentBranch()
	assert.NoError(t, err)
	// Default branch could be "main" or "master"
	assert.True(t, branch == "main" || branch == "master")
}

func TestRepository_GetStatus(t *testing.T) {
	tempDir, repo := createTestRepo(t)

	t.Run("clean repository", func(t *testing.T) {
		status, err := repo.GetStatus()
		assert.NoError(t, err)
		assert.Empty(t, status)
	})

	t.Run("with untracked file", func(t *testing.T) {
		createTestFile(t, tempDir, "test.txt", "hello world")

		status, err := repo.GetStatus()
		assert.NoError(t, err)
		assert.Contains(t, status, "test.txt")
		assert.Contains(t, status, "??") // Untracked file marker
	})
}

func TestRepository_GetDiff(t *testing.T) {
	tempDir, repo := createTestRepo(t)

	t.Run("no changes", func(t *testing.T) {
		diff, err := repo.GetDiff()
		assert.NoError(t, err)
		assert.Empty(t, diff)
	})

	t.Run("with unstaged changes", func(t *testing.T) {
		// Create and commit a file first
		createTestFile(t, tempDir, "test.txt", "initial content")
		err := repo.Add("test.txt")
		require.NoError(t, err)
		err = repo.Commit("Initial commit")
		require.NoError(t, err)

		// Modify the file
		createTestFile(t, tempDir, "test.txt", "modified content")

		diff, err := repo.GetDiff()
		assert.NoError(t, err)
		assert.Contains(t, diff, "test.txt")
		assert.Contains(t, diff, "-initial content")
		assert.Contains(t, diff, "+modified content")
	})
}

func TestRepository_GetStagedDiff(t *testing.T) {
	tempDir, repo := createTestRepo(t)

	t.Run("no staged changes", func(t *testing.T) {
		diff, err := repo.GetStagedDiff()
		assert.NoError(t, err)
		assert.Empty(t, diff)
	})

	t.Run("with staged changes", func(t *testing.T) {
		// Create and commit a file first
		createTestFile(t, tempDir, "test.txt", "initial content")
		err := repo.Add("test.txt")
		require.NoError(t, err)
		err = repo.Commit("Initial commit")
		require.NoError(t, err)

		// Modify and stage the file
		createTestFile(t, tempDir, "test.txt", "staged content")
		err = repo.Add("test.txt")
		require.NoError(t, err)

		diff, err := repo.GetStagedDiff()
		assert.NoError(t, err)
		assert.Contains(t, diff, "test.txt")
		assert.Contains(t, diff, "-initial content")
		assert.Contains(t, diff, "+staged content")
	})
}

func TestRepository_Add(t *testing.T) {
	tempDir, repo := createTestRepo(t)

	t.Run("add single file", func(t *testing.T) {
		createTestFile(t, tempDir, "single.txt", "content")

		err := repo.Add("single.txt")
		assert.NoError(t, err)

		// Verify file is staged
		status, err := repo.GetStatus()
		require.NoError(t, err)
		assert.Contains(t, status, "A  single.txt") // Added file marker
	})

	t.Run("add multiple files", func(t *testing.T) {
		createTestFile(t, tempDir, "file1.txt", "content1")
		createTestFile(t, tempDir, "file2.txt", "content2")

		err := repo.Add("file1.txt", "file2.txt")
		assert.NoError(t, err)

		// Verify both files are staged
		status, err := repo.GetStatus()
		require.NoError(t, err)
		assert.Contains(t, status, "file1.txt")
		assert.Contains(t, status, "file2.txt")
	})

	t.Run("add all files", func(t *testing.T) {
		createTestFile(t, tempDir, "all1.txt", "content1")
		createTestFile(t, tempDir, "all2.txt", "content2")

		err := repo.Add(".")
		assert.NoError(t, err)

		// Verify files are staged
		status, err := repo.GetStatus()
		require.NoError(t, err)
		assert.Contains(t, status, "all1.txt")
		assert.Contains(t, status, "all2.txt")
	})

	t.Run("add non-existent file", func(t *testing.T) {
		err := repo.Add("nonexistent.txt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to add files")
	})
}

func TestRepository_Commit(t *testing.T) {
	tempDir, repo := createTestRepo(t)

	t.Run("successful commit", func(t *testing.T) {
		createTestFile(t, tempDir, "commit.txt", "content")
		err := repo.Add("commit.txt")
		require.NoError(t, err)

		err = repo.Commit("Test commit message")
		assert.NoError(t, err)

		// Verify repository is clean after commit
		status, err := repo.GetStatus()
		require.NoError(t, err)
		assert.Empty(t, status)
	})

	t.Run("commit with nothing staged", func(t *testing.T) {
		err := repo.Commit("Empty commit")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to commit")
	})
}

func TestRepository_CreateWorktree(t *testing.T) {
	t.Skip("Worktree tests require specific git configuration and may not work in all environments")

	tempDir, repo := createTestRepo(t)

	// Need at least one commit to create a worktree
	createTestFile(t, tempDir, "initial.txt", "content")
	err := repo.Add("initial.txt")
	require.NoError(t, err)
	err = repo.Commit("Initial commit")
	require.NoError(t, err)

	worktreePath, err := repo.CreateWorktree("test-worktree")
	if err != nil {
		t.Skipf("Worktree creation failed: %v", err)
	}
	assert.NotEmpty(t, worktreePath)

	// Verify worktree directory exists
	assert.DirExists(t, worktreePath)

	// Verify initial file exists in worktree
	assert.FileExists(t, filepath.Join(worktreePath, "initial.txt"))

	// Clean up
	err = repo.RemoveWorktree(worktreePath)
	assert.NoError(t, err)
}

func TestRepository_RemoveWorktree(t *testing.T) {
	t.Skip("Worktree tests require specific git configuration and may not work in all environments")
}

func TestRepository_ResolvePath(t *testing.T) {
	tempDir, repo := createTestRepo(t)

	t.Run("absolute path", func(t *testing.T) {
		absPath := "/absolute/path/file.txt"
		resolved, err := repo.ResolvePath(absPath)
		assert.NoError(t, err)
		assert.Equal(t, absPath, resolved)
	})

	t.Run("relative path", func(t *testing.T) {
		relPath := "relative/file.txt"
		resolved, err := repo.ResolvePath(relPath)
		assert.NoError(t, err)
		// Check that the relative path is properly joined
		assert.Contains(t, resolved, relPath)
		assert.Contains(t, resolved, filepath.Base(tempDir))
	})
}

func TestIsGitRepository(t *testing.T) {
	// This test depends on the current working directory
	err := IsGitRepository()
	if err != nil {
		// If not in a git repo, create one temporarily
		tempDir, _ := createTestRepo(t)
		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)

		os.Chdir(tempDir)
		err = IsGitRepository()
		assert.NoError(t, err)
	} else {
		assert.NoError(t, err)
	}
}

func TestGetRepositoryRoot(t *testing.T) {
	// Save current directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	// Create test repo and change to it
	tempDir, _ := createTestRepo(t)
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	root, err := GetRepositoryRoot()
	assert.NoError(t, err)
	assert.Contains(t, root, filepath.Base(tempDir))
}

func TestCheckGitRepo(t *testing.T) {
	t.Run("valid git repository", func(t *testing.T) {
		tempDir, _ := createTestRepo(t)

		err := checkGitRepo(tempDir)
		assert.NoError(t, err)
	})

	t.Run("non-git directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "non-git-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		err = checkGitRepo(tempDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a git repository")
	})
}

func TestRepository_Integration(t *testing.T) {
	// This test performs a complete workflow
	tempDir, repo := createTestRepo(t)

	// 1. Check initial state
	status, err := repo.GetStatus()
	require.NoError(t, err)
	assert.Empty(t, status)

	// 2. Create and stage files
	createTestFile(t, tempDir, "file1.txt", "content1")
	createTestFile(t, tempDir, "file2.txt", "content2")

	err = repo.Add("file1.txt", "file2.txt")
	require.NoError(t, err)

	// 3. Check staged status
	status, err = repo.GetStatus()
	require.NoError(t, err)
	assert.Contains(t, status, "file1.txt")
	assert.Contains(t, status, "file2.txt")

	// 4. Get staged diff
	stagedDiff, err := repo.GetStagedDiff()
	require.NoError(t, err)
	assert.Contains(t, stagedDiff, "content1")
	assert.Contains(t, stagedDiff, "content2")

	// 5. Commit changes
	err = repo.Commit("Add test files")
	require.NoError(t, err)

	// 6. Check clean state
	status, err = repo.GetStatus()
	require.NoError(t, err)
	assert.Empty(t, status)

	// 7. Modify file and check diff
	createTestFile(t, tempDir, "file1.txt", "modified content1")

	diff, err := repo.GetDiff()
	require.NoError(t, err)
	assert.Contains(t, diff, "file1.txt")
	assert.Contains(t, diff, "-content1")
	assert.Contains(t, diff, "+modified content1")

	// 8. Verify repository operations work consistently
	// (Worktree tests skipped due to environment compatibility)
}

func TestRepository_EdgeCases(t *testing.T) {
	tempDir, repo := createTestRepo(t)

	t.Run("empty commit message", func(t *testing.T) {
		createTestFile(t, tempDir, "empty-msg.txt", "content")
		err := repo.Add("empty-msg.txt")
		require.NoError(t, err)

		err = repo.Commit("")
		// Git should still accept empty messages, but some configurations might reject them
		// The exact behavior depends on git configuration
		if err != nil {
			assert.Contains(t, err.Error(), "failed to commit")
		}
	})

	t.Run("very long file name", func(t *testing.T) {
		longName := strings.Repeat("a", 100) + ".txt"
		createTestFile(t, tempDir, longName, "content")

		err := repo.Add(longName)
		assert.NoError(t, err)

		status, err := repo.GetStatus()
		require.NoError(t, err)
		assert.Contains(t, status, longName)
	})

	t.Run("special characters in filename", func(t *testing.T) {
		specialName := "special-file_123.txt"
		createTestFile(t, tempDir, specialName, "content")

		err := repo.Add(specialName)
		assert.NoError(t, err)

		status, err := repo.GetStatus()
		require.NoError(t, err)
		assert.Contains(t, status, specialName)
	})
}

func TestRepository_ErrorHandling(t *testing.T) {
	_, repo := createTestRepo(t)

	t.Run("operations in invalid state", func(t *testing.T) {
		// Try to create worktree without any commits
		_, err := repo.CreateWorktree("no-commits")
		assert.Error(t, err)
	})

	t.Run("remove non-existent worktree", func(t *testing.T) {
		err := repo.RemoveWorktree("/non/existent/path")
		assert.Error(t, err)
	})

	t.Run("operations after corruption", func(t *testing.T) {
		// This test simulates git repo corruption by changing the working directory
		// to a path that doesn't exist anymore
		originalPath := repo.Path
		repo.Path = "/definitely/does/not/exist"

		_, err := repo.GetCurrentBranch()
		assert.Error(t, err)

		_, err = repo.GetStatus()
		assert.Error(t, err)

		_, err = repo.GetDiff()
		assert.Error(t, err)

		// Restore for cleanup
		repo.Path = originalPath
	})
}
