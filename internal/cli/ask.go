package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
	"github.com/spf13/cobra"
)

// AskCommand implements the ask command
type AskCommand struct {
	*BaseCommand
	Question string
}

// NewAskCommand creates a new ask command
func NewAskCommand() *AskCommand {
	return &AskCommand{
		BaseCommand: NewBaseCommand(
			"ask",
			"Ask a question about code",
			`Ask a question about code in files, directories, or from stdin.
The LLM will analyze the code and provide an answer.

Examples:
  sigil ask "What does this function do?" --file main.go
  sigil ask "How can I optimize this code?" --dir src/
  sigil ask "Explain this algorithm" --git --staged`,
		),
	}
}

// Execute runs the ask command
func (c *AskCommand) Execute(ctx context.Context, args []string) error {
	start := time.Now()

	// Validate arguments
	if len(args) == 0 {
		return errors.ValidationError("Execute", "question is required")
	}

	c.Question = strings.Join(args, " ")

	// Run pre-checks
	if err := c.RunPreChecks(); err != nil {
		return err
	}

	// Get input
	inputHandler := NewInputHandler(c.GetCommonFlags())
	inputCtx, err := inputHandler.GetInput()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInput, "Execute", "failed to get input")
	}

	// Get model
	mdl, err := c.GetModel(ctx)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeModel, "Execute", "failed to get model")
	}

	// Get memory context if requested
	memoryCtx, err := inputHandler.GetMemoryContext()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInput, "Execute", "failed to get memory context")
	}

	// Build prompt
	promptInput := c.buildPrompt(inputCtx, memoryCtx)

	logger.Debug("executing ask command", "question", c.Question, "input_type", inputCtx.InputType)

	// Run model
	response, err := mdl.RunPrompt(ctx, promptInput)
	if err != nil {
		duration := time.Since(start)
		output := CreateErrorOutput("ask", err, duration)
		outputHandler := NewOutputHandler(c.GetCommonFlags())
		outputHandler.WriteOutput(output)
		return errors.Wrap(err, errors.ErrorTypeModel, "Execute", "model execution failed")
	}

	// Create output
	duration := time.Since(start)
	output := CreateOutput("ask", inputCtx, response, duration)

	// Write output
	outputHandler := NewOutputHandler(c.GetCommonFlags())
	if err := outputHandler.WriteOutput(output); err != nil {
		return errors.Wrap(err, errors.ErrorTypeOutput, "Execute", "failed to write output")
	}

	return nil
}

// GetCobraCommand returns the cobra command for the ask command
func (c *AskCommand) GetCobraCommand() *cobra.Command {
	cmd := c.BaseCommand.GetCobraCommand()

	cmd.Use = "ask [question]"
	cmd.Args = cobra.MinimumNArgs(1)
	cmd.RunE = func(cobraCmd *cobra.Command, args []string) error {
		return c.Execute(context.Background(), args)
	}

	return cmd
}

// buildPrompt builds the prompt for the model
func (c *AskCommand) buildPrompt(inputCtx *CommandContext, memoryCtx []model.MemoryEntry) model.PromptInput {
	var systemPrompt strings.Builder

	systemPrompt.WriteString("You are an AI assistant specialized in code analysis and explanation. ")
	systemPrompt.WriteString("Your task is to answer questions about code accurately and helpfully. ")
	systemPrompt.WriteString("Provide clear, concise explanations that are appropriate for the user's level of understanding.")

	var userPrompt strings.Builder
	userPrompt.WriteString(fmt.Sprintf("Question: %s\n\n", c.Question))

	// Add context based on input type
	switch inputCtx.InputType {
	case InputTypeFile:
		if len(inputCtx.Files) > 0 {
			userPrompt.WriteString(fmt.Sprintf("File: %s\n", inputCtx.Files[0].Path))
		}
		userPrompt.WriteString("Code:\n```\n")
		userPrompt.WriteString(inputCtx.Input)
		userPrompt.WriteString("\n```\n")

	case InputTypeDirectory:
		userPrompt.WriteString("Code from directory:\n")
		userPrompt.WriteString(inputCtx.Input)

	case InputTypeGitDiff:
		userPrompt.WriteString("Git diff:\n```diff\n")
		userPrompt.WriteString(inputCtx.Input)
		userPrompt.WriteString("\n```\n")

	case InputTypeText:
		userPrompt.WriteString("Context:\n")
		userPrompt.WriteString(inputCtx.Input)

	default:
		userPrompt.WriteString("Context:\n")
		userPrompt.WriteString(inputCtx.Input)
	}

	// Build file content for model
	files := make([]model.FileContent, 0, len(inputCtx.Files))
	for _, file := range inputCtx.Files {
		files = append(files, model.FileContent{
			Path:    file.Path,
			Content: file.Content,
			Type:    "code",
		})
	}

	return model.PromptInput{
		SystemPrompt: systemPrompt.String(),
		UserPrompt:   userPrompt.String(),
		Files:        files,
		Memory:       memoryCtx,
		MaxTokens:    4000,
		Temperature:  0.1, // Lower temperature for more focused responses
	}
}

// Create the global ask command instance
var askCmd = NewAskCommand().GetCobraCommand()
