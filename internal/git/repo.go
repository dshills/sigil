package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repository represents a Git repository.
type Repository struct {
	Path string
}

// NewRepository creates a new Repository instance.
func NewRepository(path string) (*Repository, error) {
	// If no path provided, use current directory
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	// Check if it's a git repository
	if err := checkGitRepo(path); err != nil {
		return nil, err
	}

	return &Repository{Path: path}, nil
}

// checkGitRepo verifies that the given path is inside a git repository.
func checkGitRepo(path string) error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("not a git repository (or any of the parent directories)")
	}

	// Verify we got a valid response
	gitDir := strings.TrimSpace(string(output))
	if gitDir == "" {
		return fmt.Errorf("unable to determine git directory")
	}

	return nil
}

// GetRoot returns the root directory of the git repository.
func (r *Repository) GetRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = r.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get repository root: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetCurrentBranch returns the current branch name
func (r *Repository) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = r.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetStatus returns the working tree status
func (r *Repository) GetStatus() (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = r.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	return string(output), nil
}

// GetDiff returns the diff of unstaged changes
func (r *Repository) GetDiff() (string, error) {
	cmd := exec.Command("git", "diff")
	cmd.Dir = r.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return string(output), nil
}

// GetStagedDiff returns the diff of staged changes
func (r *Repository) GetStagedDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--staged")
	cmd.Dir = r.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get staged diff: %w", err)
	}

	return string(output), nil
}

// Add stages files
func (r *Repository) Add(files ...string) error {
	args := append([]string{"add"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add files: %w", err)
	}

	return nil
}

// Commit creates a commit with the given message
func (r *Repository) Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = r.Path

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

// CreateWorktree creates a new worktree for sandboxed operations
func (r *Repository) CreateWorktree(name string) (string, error) {
	// Create temporary directory for worktree
	tmpDir, err := os.MkdirTemp("", "sigil-worktree-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Get current branch
	branch, err := r.GetCurrentBranch()
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	// Create worktree
	cmd := exec.Command("git", "worktree", "add", tmpDir, branch)
	cmd.Dir = r.Path

	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to create worktree: %w", err)
	}

	return tmpDir, nil
}

// RemoveWorktree removes a worktree
func (r *Repository) RemoveWorktree(path string) error {
	// First, remove the worktree from git
	cmd := exec.Command("git", "worktree", "remove", path, "--force")
	cmd.Dir = r.Path

	if err := cmd.Run(); err != nil {
		// If git command fails, still try to remove the directory
		os.RemoveAll(path)
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	// Ensure the directory is removed
	os.RemoveAll(path)
	return nil
}

// IsGitRepository checks if the current directory is a git repository
func IsGitRepository() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	return checkGitRepo(cwd)
}

// GetRepositoryRoot returns the root directory of the current git repository
func GetRepositoryRoot() (string, error) {
	repo, err := NewRepository("")
	if err != nil {
		return "", err
	}

	return repo.GetRoot()
}

// ResolvePath resolves a path relative to the repository root
func (r *Repository) ResolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	root, err := r.GetRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, path), nil
}
