package git

import (
	"fmt"
	"os"
	"path/filepath"
)

// Worktree represents a Git worktree for sandboxed operations
type Worktree struct {
	repo *Repository
	path string
}

// NewWorktree creates a new worktree
func NewWorktree(repo *Repository, name string) (*Worktree, error) {
	path, err := repo.CreateWorktree(name)
	if err != nil {
		return nil, err
	}

	return &Worktree{
		repo: repo,
		path: path,
	}, nil
}

// Path returns the worktree path
func (w *Worktree) Path() string {
	return w.path
}

// Cleanup removes the worktree
func (w *Worktree) Cleanup() error {
	return w.repo.RemoveWorktree(w.path)
}

// CopyFile copies a file from the main repository to the worktree
func (w *Worktree) CopyFile(relativePath string) error {
	srcPath, err := w.repo.ResolvePath(relativePath)
	if err != nil {
		return fmt.Errorf("failed to resolve source path: %w", err)
	}

	dstPath := filepath.Join(w.path, relativePath)

	// Create destination directory if needed
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source file
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Write to destination
	if err := os.WriteFile(dstPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}

// ApplyChanges applies changes from the worktree back to the main repository
func (w *Worktree) ApplyChanges(files []string) error {
	for _, file := range files {
		srcPath := filepath.Join(w.path, file)
		dstPath, err := w.repo.ResolvePath(file)
		if err != nil {
			return fmt.Errorf("failed to resolve destination path for %s: %w", file, err)
		}

		// Read from worktree
		content, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s from worktree: %w", file, err)
		}

		// Write to main repository
		if err := os.WriteFile(dstPath, content, 0600); err != nil {
			return fmt.Errorf("failed to write file %s to repository: %w", file, err)
		}
	}

	return nil
}
