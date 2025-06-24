package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// DiffOptions represents options for generating diffs.
type DiffOptions struct {
	Staged bool
	Files  []string
}

// Diff generates a git diff based on options.
func (r *Repository) Diff(opts DiffOptions) (string, error) {
	args := []string{"diff"}

	if opts.Staged {
		args = append(args, "--staged")
	}

	if len(opts.Files) > 0 {
		args = append(args, "--")
		args = append(args, opts.Files...)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to generate diff: %w", err)
	}

	return string(output), nil
}

// ApplyPatch applies a patch to the repository.
func (r *Repository) ApplyPatch(patch string) error {
	cmd := exec.Command("git", "apply", "-")
	cmd.Dir = r.Path
	cmd.Stdin = strings.NewReader(patch)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply patch: %s", string(output))
	}

	return nil
}

// CheckPatch checks if a patch can be applied cleanly.
func (r *Repository) CheckPatch(patch string) error {
	cmd := exec.Command("git", "apply", "--check", "-")
	cmd.Dir = r.Path
	cmd.Stdin = strings.NewReader(patch)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("patch cannot be applied cleanly: %s", string(output))
	}

	return nil
}

// GeneratePatch generates a patch for the given files.
func (r *Repository) GeneratePatch(files []string) (string, error) {
	args := []string{"diff", "--no-index", "--"}
	args = append(args, files...)

	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		// git diff returns exit code 1 when there are differences
		// This is expected behavior, so we check if we got output
		if len(output) == 0 {
			return "", fmt.Errorf("failed to generate patch: %w", err)
		}
	}

	return string(output), nil
}
