// Package sandbox provides safe execution environments using Git worktrees
package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/git"
	"github.com/dshills/sigil/internal/logger"
)

// WorktreeManager manages Git worktrees for sandbox execution
type WorktreeManager struct {
	repo      *git.Repository
	baseDir   string
	worktrees map[string]*Worktree
}

// Worktree represents a Git worktree sandbox
type Worktree struct {
	ID        string
	Path      string
	Branch    string
	CreatedAt time.Time
	LastUsed  time.Time
	manager   *WorktreeManager
}

// NewWorktreeManager creates a new worktree manager
func NewWorktreeManager(repo *git.Repository) (*WorktreeManager, error) {
	baseDir := filepath.Join(".sigil", "sandbox")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "NewWorktreeManager", "failed to create sandbox directory")
	}

	return &WorktreeManager{
		repo:      repo,
		baseDir:   baseDir,
		worktrees: make(map[string]*Worktree),
	}, nil
}

// CreateWorktree creates a new Git worktree for sandbox execution
func (wm *WorktreeManager) CreateWorktree(branchName string) (*Worktree, error) {
	// Generate unique ID
	id := generateWorktreeID()
	worktreePath := filepath.Join(wm.baseDir, id)

	logger.Debug("creating worktree", "id", id, "path", worktreePath, "branch", branchName)

	// Note: Fetch functionality not implemented in git.Repository yet
	// Continue with local state

	// Create the worktree
	cmd := exec.Command("git", "worktree", "add", "-b", fmt.Sprintf("sigil-sandbox-%s", id), worktreePath, branchName)
	rootPath, err := wm.repo.GetRoot()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeGit, "CreateWorktree", "failed to get repository root")
	}
	cmd.Dir = rootPath

	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeGit, "CreateWorktree",
			fmt.Sprintf("failed to create worktree: %s", string(output)))
	}

	worktree := &Worktree{
		ID:        id,
		Path:      worktreePath,
		Branch:    fmt.Sprintf("sigil-sandbox-%s", id),
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		manager:   wm,
	}

	wm.worktrees[id] = worktree

	logger.Info("created sandbox worktree", "id", id, "path", worktreePath)
	return worktree, nil
}

// GetWorktree retrieves an existing worktree by ID
func (wm *WorktreeManager) GetWorktree(id string) (*Worktree, error) {
	worktree, exists := wm.worktrees[id]
	if !exists {
		return nil, errors.New(errors.ErrorTypeInput, "GetWorktree",
			fmt.Sprintf("worktree %s not found", id))
	}

	worktree.LastUsed = time.Now()
	return worktree, nil
}

// ListWorktrees returns all active worktrees
func (wm *WorktreeManager) ListWorktrees() []*Worktree {
	worktrees := make([]*Worktree, 0, len(wm.worktrees))
	for _, wt := range wm.worktrees {
		worktrees = append(worktrees, wt)
	}
	return worktrees
}

// CleanupWorktree removes a worktree and cleans up resources
func (wm *WorktreeManager) CleanupWorktree(id string) error {
	worktree, exists := wm.worktrees[id]
	if !exists {
		return errors.New(errors.ErrorTypeInput, "CleanupWorktree",
			fmt.Sprintf("worktree %s not found", id))
	}

	logger.Debug("cleaning up worktree", "id", id, "path", worktree.Path)

	// Remove the worktree
	cmd := exec.Command("git", "worktree", "remove", "--force", worktree.Path)
	rootPath, err := wm.repo.GetRoot()
	if err != nil {
		logger.Warn("failed to get repository root", "error", err)
		return err
	}
	cmd.Dir = rootPath

	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("failed to remove worktree", "id", id, "error", err, "output", string(output))
		// Try manual cleanup
		if err := os.RemoveAll(worktree.Path); err != nil {
			return errors.Wrap(err, errors.ErrorTypeFS, "CleanupWorktree",
				"failed to remove worktree directory")
		}
	}

	// Delete the branch
	cmd = exec.Command("git", "branch", "-D", worktree.Branch)
	rootPath, err = wm.repo.GetRoot()
	if err != nil {
		logger.Warn("failed to get repository root for branch deletion", "error", err)
		// Continue with cleanup anyway
	} else {
		cmd.Dir = rootPath
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("failed to delete sandbox branch", "branch", worktree.Branch, "error", err, "output", string(output))
		// Non-critical error, continue
	}

	delete(wm.worktrees, id)

	logger.Info("cleaned up sandbox worktree", "id", id)
	return nil
}

// CleanupOldWorktrees removes worktrees older than the specified age
func (wm *WorktreeManager) CleanupOldWorktrees(maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)
	var toDelete []string

	for id, worktree := range wm.worktrees {
		if worktree.LastUsed.Before(cutoff) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		if err := wm.CleanupWorktree(id); err != nil {
			logger.Warn("failed to cleanup old worktree", "id", id, "error", err)
			// Continue with other worktrees
		}
	}

	if len(toDelete) > 0 {
		logger.Info("cleaned up old worktrees", "count", len(toDelete))
	}

	return nil
}

// Worktree methods

// Execute runs a command in the worktree
func (wt *Worktree) Execute(command string, args ...string) (*ExecutionResult, error) {
	wt.LastUsed = time.Now()

	logger.Debug("executing command in worktree", "id", wt.ID, "command", command, "args", args)

	cmd := exec.Command(command, args...)
	cmd.Dir = wt.Path

	// Set environment to ensure Git operations work correctly
	cmd.Env = os.Environ()

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	result := &ExecutionResult{
		Command:    fmt.Sprintf("%s %s", command, strings.Join(args, " ")),
		Output:     string(output),
		ExitCode:   0,
		WorktreeID: wt.ID,
		Timestamp:  time.Now(),
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = err.Error()
	}

	logger.Debug("command executed", "id", wt.ID, "exit_code", result.ExitCode, "output_length", len(output))
	return result, nil
}

// WriteFile writes content to a file in the worktree
func (wt *Worktree) WriteFile(relativePath string, content []byte) error {
	wt.LastUsed = time.Now()

	fullPath := filepath.Join(wt.Path, relativePath)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "WriteFile", "failed to create directory")
	}

	if err := os.WriteFile(fullPath, content, 0600); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "WriteFile", "failed to write file")
	}

	logger.Debug("wrote file in worktree", "id", wt.ID, "path", relativePath, "size", len(content))
	return nil
}

// ReadFile reads content from a file in the worktree
func (wt *Worktree) ReadFile(relativePath string) ([]byte, error) {
	wt.LastUsed = time.Now()

	fullPath := filepath.Join(wt.Path, relativePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "ReadFile", "failed to read file")
	}

	logger.Debug("read file from worktree", "id", wt.ID, "path", relativePath, "size", len(content))
	return content, nil
}

// GetChanges returns the Git diff of changes in the worktree
func (wt *Worktree) GetChanges() (string, error) {
	wt.LastUsed = time.Now()

	cmd := exec.Command("git", "diff", "--no-index", "/dev/null", ".")
	cmd.Dir = wt.Path

	// Use git diff to show all changes
	cmd = exec.Command("git", "diff", "HEAD")
	cmd.Dir = wt.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Git diff returns non-zero exit code when there are differences
		// This is expected behavior, so we only error on actual failures
		if !strings.Contains(err.Error(), "exit status") {
			return "", errors.Wrap(err, errors.ErrorTypeGit, "GetChanges", "failed to get diff")
		}
	}

	return string(output), nil
}

// Commit commits changes in the worktree
func (wt *Worktree) Commit(message string) error {
	wt.LastUsed = time.Now()

	// Add all changes
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = wt.Path
	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "Commit",
			fmt.Sprintf("failed to add changes: %s", string(output)))
	}

	// Commit changes
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = wt.Path
	if output, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "Commit",
			fmt.Sprintf("failed to commit changes: %s", string(output)))
	}

	logger.Debug("committed changes in worktree", "id", wt.ID, "message", message)
	return nil
}

// Cleanup removes this worktree
func (wt *Worktree) Cleanup() error {
	return wt.manager.CleanupWorktree(wt.ID)
}

// ExecutionResult represents the result of a command execution
type ExecutionResult struct {
	Command    string    `json:"command"`
	Output     string    `json:"output"`
	Error      string    `json:"error,omitempty"`
	ExitCode   int       `json:"exit_code"`
	WorktreeID string    `json:"worktree_id"`
	Timestamp  time.Time `json:"timestamp"`
}

// Success returns true if the command executed successfully
func (er *ExecutionResult) Success() bool {
	return er.ExitCode == 0
}

// generateWorktreeID generates a unique identifier for a worktree
func generateWorktreeID() string {
	return fmt.Sprintf("%d-%s", time.Now().Unix(), randomString(8))
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
