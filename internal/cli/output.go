// Package cli provides output formatting for Sigil commands
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
)

// OutputHandler handles various output formats
type OutputHandler struct {
	flags CommonFlags
}

// NewOutputHandler creates a new output handler
func NewOutputHandler(flags CommonFlags) *OutputHandler {
	return &OutputHandler{
		flags: flags,
	}
}

// CommandOutput represents the output of a command
type CommandOutput struct {
	// Content is the main output content
	Content string `json:"content"`

	// Command metadata
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`

	// Input information
	InputType  string   `json:"input_type"`
	InputFiles []string `json:"input_files,omitempty"`

	// Model information
	Model      string `json:"model"`
	TokensUsed int    `json:"tokens_used"`

	// Processing metadata
	Duration time.Duration `json:"duration"`
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`

	// Additional context
	Files map[string]string `json:"files,omitempty"`
	Patch string            `json:"patch,omitempty"`
}

// WriteOutput writes the output in the specified format
func (h *OutputHandler) WriteOutput(output *CommandOutput) error {
	var writer io.Writer = os.Stdout

	// Handle output file
	if h.flags.Out != "" {
		file, err := os.Create(h.flags.Out)
		if err != nil {
			return errors.Wrap(err, errors.ErrorTypeFS, "WriteOutput", "failed to create output file")
		}
		defer file.Close()
		writer = file
	}

	// Format output based on flags
	switch {
	case h.flags.JSON:
		return h.writeJSON(writer, output)
	case h.flags.Patch:
		return h.writePatch(writer, output)
	case h.flags.InPlace:
		return h.writeInPlace(output)
	default:
		return h.writeText(writer, output)
	}
}

// writeJSON writes output in JSON format
func (h *OutputHandler) writeJSON(writer io.Writer, output *CommandOutput) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(output); err != nil {
		return errors.Wrap(err, errors.ErrorTypeOutput, "writeJSON", "failed to encode JSON")
	}

	return nil
}

// writePatch writes output as a patch
func (h *OutputHandler) writePatch(writer io.Writer, output *CommandOutput) error {
	if output.Patch == "" {
		// Generate patch from content if not explicitly provided
		patch := h.generatePatch(output)
		if patch == "" {
			return errors.New(errors.ErrorTypeOutput, "writePatch", "no patch content available")
		}
		output.Patch = patch
	}

	_, err := fmt.Fprint(writer, output.Patch)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeOutput, "writePatch", "failed to write patch")
	}

	return nil
}

// writeInPlace writes output directly to input files
func (h *OutputHandler) writeInPlace(output *CommandOutput) error {
	if len(output.Files) == 0 {
		return errors.New(errors.ErrorTypeOutput, "writeInPlace", "no files to update")
	}

	for filePath, content := range output.Files {
		logger.Debug("writing file in place", "path", filePath)

		if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
			return errors.Wrap(err, errors.ErrorTypeFS, "writeInPlace",
				fmt.Sprintf("failed to write file %s", filePath))
		}
	}

	// Also output a summary to stdout
	fmt.Printf("Updated %d file(s)\n", len(output.Files))
	for filePath := range output.Files {
		fmt.Printf("  %s\n", filePath)
	}

	return nil
}

// writeText writes output in plain text format
func (h *OutputHandler) writeText(writer io.Writer, output *CommandOutput) error {
	_, err := fmt.Fprint(writer, output.Content)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeOutput, "writeText", "failed to write text")
	}

	return nil
}

// generatePatch generates a unified diff patch format
func (h *OutputHandler) generatePatch(output *CommandOutput) string {
	if len(output.Files) == 0 {
		return ""
	}

	var patch strings.Builder

	for filePath, newContent := range output.Files {
		// Read original file content
		originalContent, err := os.ReadFile(filePath)
		if err != nil {
			logger.Warn("failed to read original file for patch", "path", filePath, "error", err)
			continue
		}

		// Generate unified diff
		diff := h.generateUnifiedDiff(string(originalContent), newContent, filePath)
		if diff != "" {
			patch.WriteString(diff)
			patch.WriteString("\n")
		}
	}

	return patch.String()
}

// generateUnifiedDiff generates a unified diff between two strings
func (h *OutputHandler) generateUnifiedDiff(original, modified, filename string) string {
	// Simple implementation - in practice, you'd use a proper diff library
	if original == modified {
		return ""
	}

	var diff strings.Builder

	// Header
	diff.WriteString(fmt.Sprintf("--- a/%s\n", filename))
	diff.WriteString(fmt.Sprintf("+++ b/%s\n", filename))

	originalLines := strings.Split(original, "\n")
	modifiedLines := strings.Split(modified, "\n")

	// Simple line-by-line diff (not optimal, but functional)
	maxLines := len(originalLines)
	if len(modifiedLines) > maxLines {
		maxLines = len(modifiedLines)
	}

	diff.WriteString("@@ -1,")
	diff.WriteString(fmt.Sprintf("%d +1,%d @@\n", len(originalLines), len(modifiedLines)))

	for i := 0; i < maxLines; i++ {
		if i < len(originalLines) && i < len(modifiedLines) {
			if originalLines[i] != modifiedLines[i] {
				diff.WriteString(fmt.Sprintf("-%s\n", originalLines[i]))
				diff.WriteString(fmt.Sprintf("+%s\n", modifiedLines[i]))
			} else {
				diff.WriteString(fmt.Sprintf(" %s\n", originalLines[i]))
			}
		} else if i < len(originalLines) {
			diff.WriteString(fmt.Sprintf("-%s\n", originalLines[i]))
		} else if i < len(modifiedLines) {
			diff.WriteString(fmt.Sprintf("+%s\n", modifiedLines[i]))
		}
	}

	return diff.String()
}

// CreateOutput creates a CommandOutput from a model response
func CreateOutput(command string, input *CommandContext, response model.PromptOutput, duration time.Duration) *CommandOutput {
	output := &CommandOutput{
		Content:    response.Response,
		Command:    command,
		Timestamp:  time.Now(),
		Duration:   duration,
		Success:    true,
		Model:      response.Model,
		TokensUsed: response.TokensUsed,
	}

	// Set input type
	switch input.InputType {
	case InputTypeFile:
		output.InputType = "file"
		if len(input.Files) > 0 {
			output.InputFiles = []string{input.Files[0].Path}
		}
	case InputTypeDirectory:
		output.InputType = "directory"
		for _, file := range input.Files {
			output.InputFiles = append(output.InputFiles, file.Path)
		}
	case InputTypeGitDiff:
		output.InputType = "git-diff"
		for _, file := range input.Files {
			output.InputFiles = append(output.InputFiles, file.Path)
		}
	case InputTypeText:
		output.InputType = "text"
	default:
		output.InputType = "text"
	}

	return output
}

// CreateErrorOutput creates a CommandOutput for error cases
func CreateErrorOutput(command string, err error, duration time.Duration) *CommandOutput {
	return &CommandOutput{
		Command:   command,
		Timestamp: time.Now(),
		Duration:  duration,
		Success:   false,
		Error:     err.Error(),
	}
}
