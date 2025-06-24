// Package cli provides input handling for Sigil commands
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/git"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
)

// InputHandler handles various input sources
type InputHandler struct {
	flags CommonFlags
}

// NewInputHandler creates a new input handler
func NewInputHandler(flags CommonFlags) *InputHandler {
	return &InputHandler{
		flags: flags,
	}
}

// GetInput retrieves input based on flags
func (h *InputHandler) GetInput() (*CommandContext, error) {
	ctx := &CommandContext{
		Files: []FileInput{},
	}

	switch {
	case h.flags.File != "":
		return h.handleFileInput(ctx)
	case h.flags.Dir != "":
		return h.handleDirectoryInput(ctx)
	case h.flags.Git:
		return h.handleGitInput(ctx)
	case h.flags.Stdin:
		return h.handleStdinInput(ctx)
	default:
		// No explicit input source, check for args
		return nil, errors.ValidationError("GetInput", "no input source specified")
	}
}

// handleFileInput handles file-based input
func (h *InputHandler) handleFileInput(ctx *CommandContext) (*CommandContext, error) {
	logger.Debug("handling file input", "file", h.flags.File)

	// Read file content
	content, err := os.ReadFile(h.flags.File)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "handleFileInput", "failed to read file")
	}

	fileInput := FileInput{
		Path:    h.flags.File,
		Content: string(content),
	}

	// Handle line range if specified
	if h.flags.Lines != "" {
		lines, err := h.parseLineRange(h.flags.Lines)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrorTypeInput, "handleFileInput", "invalid line range")
		}
		fileInput.Lines = lines
		fileInput.Content = h.extractLines(string(content), lines)
	}

	ctx.Files = append(ctx.Files, fileInput)
	ctx.InputType = InputTypeFile
	ctx.Input = fileInput.Content

	return ctx, nil
}

// handleDirectoryInput handles directory-based input
func (h *InputHandler) handleDirectoryInput(ctx *CommandContext) (*CommandContext, error) {
	logger.Debug("handling directory input", "dir", h.flags.Dir)

	// Walk directory and collect files
	err := filepath.Walk(h.flags.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			logger.Warn("failed to read file", "path", path, "error", err)
			return nil // Continue walking
		}

		ctx.Files = append(ctx.Files, FileInput{
			Path:    path,
			Content: string(content),
		})

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "handleDirectoryInput", "failed to walk directory")
	}

	ctx.InputType = InputTypeDirectory

	// Combine all file contents for the input
	var combined strings.Builder
	for _, file := range ctx.Files {
		combined.WriteString(fmt.Sprintf("=== %s ===\n", file.Path))
		combined.WriteString(file.Content)
		combined.WriteString("\n\n")
	}
	ctx.Input = combined.String()

	return ctx, nil
}

// handleGitInput handles git-based input
func (h *InputHandler) handleGitInput(ctx *CommandContext) (*CommandContext, error) {
	logger.Debug("handling git input", "staged", h.flags.Staged)

	// Open repository
	repo, err := git.NewRepository(".")
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeGit, "handleGitInput", "failed to open repository")
	}

	// Get diff
	var diff string

	if h.flags.Staged {
		diff, err = repo.GetStagedDiff()
	} else {
		diff, err = repo.GetDiff()
	}

	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeGit, "handleGitInput", "failed to get diff")
	}

	ctx.InputType = InputTypeGitDiff
	ctx.Input = diff

	// Parse files from diff (simple implementation)
	files := h.parseFilesFromDiff(diff)
	for _, file := range files {
		ctx.Files = append(ctx.Files, FileInput{
			Path: file,
			// Content will be empty for diffs
		})
	}

	return ctx, nil
}

// handleStdinInput handles stdin-based input
func (h *InputHandler) handleStdinInput(ctx *CommandContext) (*CommandContext, error) {
	logger.Debug("handling stdin input")

	// Read from stdin
	reader := bufio.NewReader(os.Stdin)
	var content strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrap(err, errors.ErrorTypeInput, "handleStdinInput", "failed to read stdin")
		}
		content.WriteString(line)
	}

	ctx.InputType = InputTypeText
	ctx.Input = content.String()

	return ctx, nil
}

// parseLineRange parses a line range string (e.g., "10-20" or "15")
func (h *InputHandler) parseLineRange(rangeStr string) ([]int, error) {
	parts := strings.Split(rangeStr, "-")

	if len(parts) == 1 {
		// Single line
		line, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid line number: %s", parts[0])
		}
		return []int{line}, nil
	}

	if len(parts) == 2 {
		// Range
		start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid start line: %s", parts[0])
		}

		end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid end line: %s", parts[1])
		}

		if start > end {
			return nil, fmt.Errorf("start line %d is greater than end line %d", start, end)
		}

		lines := make([]int, 0, end-start+1)
		for i := start; i <= end; i++ {
			lines = append(lines, i)
		}
		return lines, nil
	}

	return nil, fmt.Errorf("invalid line range format: %s", rangeStr)
}

// extractLines extracts specific lines from content
func (h *InputHandler) extractLines(content string, lines []int) string {
	allLines := strings.Split(content, "\n")
	var result strings.Builder

	lineMap := make(map[int]bool)
	for _, line := range lines {
		lineMap[line] = true
	}

	for i, line := range allLines {
		lineNum := i + 1 // 1-based line numbers
		if lineMap[lineNum] {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.String()
}

// parseFilesFromDiff extracts file paths from git diff output
func (h *InputHandler) parseFilesFromDiff(diff string) []string {
	var files []string
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			// Extract file path from "diff --git a/file b/file"
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				// Remove "a/" prefix
				file := strings.TrimPrefix(parts[2], "a/")
				files = append(files, file)
			}
		}
	}

	return files
}

// GetMemoryContext retrieves memory context if requested
func (h *InputHandler) GetMemoryContext() ([]model.MemoryEntry, error) {
	if !h.flags.IncludeMemory {
		return nil, nil
	}

	// TODO: Implement memory retrieval
	logger.Debug("memory context requested", "depth", h.flags.MemoryDepth)

	return []model.MemoryEntry{}, nil
}
