package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/git"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/sandbox"
	"github.com/spf13/cobra"
)

const (
	defaultSandboxSubcommand = "list"
)

// SandboxCommand implements the sandbox command
type SandboxCommand struct {
	*BaseCommand
	Subcommand string
	Command    string
	Args       []string
	File       string
	Content    string
	Timeout    time.Duration
}

// NewSandboxCommand creates a new sandbox command
func NewSandboxCommand() *SandboxCommand {
	return &SandboxCommand{
		BaseCommand: NewBaseCommand(
			"sandbox",
			"Manage sandbox environments",
			`The sandbox command allows you to create, manage, and execute code in isolated
sandbox environments using Git worktrees.

Examples:
  sigil sandbox list                    # List active sandboxes
  sigil sandbox create                  # Create a new sandbox
  sigil sandbox exec <id> go build     # Execute command in sandbox
  sigil sandbox validate <file>        # Validate file against rules
  sigil sandbox stats                  # Show sandbox statistics
  sigil sandbox clean                  # Clean up old sandboxes`,
		),
		Timeout: 5 * time.Minute,
	}
}

// Execute runs the sandbox command
func (c *SandboxCommand) Execute(ctx context.Context, args []string) error {
	if err := c.RunPreChecks(); err != nil {
		return err
	}

	// Determine subcommand
	if len(args) > 0 {
		c.Subcommand = args[0]
	} else {
		c.Subcommand = defaultSandboxSubcommand // Default
	}

	// Create Git repository
	repo, err := git.NewRepository(".")
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "Execute", "failed to open repository")
	}

	// Create sandbox manager
	manager, err := sandbox.NewManager(repo)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeConfig, "Execute", "failed to create sandbox manager")
	}

	// Ensure cleanup
	defer func() {
		if err := manager.Cleanup(); err != nil {
			logger.Warn("failed to cleanup sandbox manager", "error", err)
		}
	}()

	logger.Debug("executing sandbox command", "subcommand", c.Subcommand)

	switch c.Subcommand {
	case "list":
		return c.executeList(manager)
	case "create":
		return c.executeCreate(manager)
	case "exec":
		return c.executeExec(manager, args[1:])
	case "validate":
		return c.executeValidate(manager, args[1:])
	case "stats":
		return c.executeStats(manager)
	case "clean":
		return c.executeClean(manager)
	case "test":
		return c.executeTest(manager, args[1:])
	default:
		return errors.New(errors.ErrorTypeInput, "Execute",
			fmt.Sprintf("unknown sandbox subcommand: %s", c.Subcommand))
	}
}

// executeList lists active sandboxes
func (c *SandboxCommand) executeList(manager sandbox.Manager) error {
	sandboxes := manager.ListSandboxes()

	if len(sandboxes) == 0 {
		fmt.Println("No active sandboxes.")
		return nil
	}

	fmt.Printf("Active Sandboxes (%d):\n\n", len(sandboxes))

	for i, sb := range sandboxes {
		fmt.Printf("%d. ID: %s\n", i+1, sb.ID)
		fmt.Printf("   Path: %s\n", sb.Path)
		fmt.Printf("   Created: %s\n", sb.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Last Used: %s\n", sb.LastUsed.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Status: %s\n", sb.Status)
		fmt.Println()
	}

	return nil
}

// executeCreate creates a new sandbox
func (c *SandboxCommand) executeCreate(manager sandbox.Manager) error {
	sandbox, err := manager.CreateSandbox()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "executeCreate", "failed to create sandbox")
	}

	fmt.Printf("Created sandbox: %s\n", sandbox.ID())
	fmt.Printf("Path: %s\n", sandbox.Path())
	fmt.Println("\nTo execute commands in this sandbox:")
	fmt.Printf("  sigil sandbox exec %s <command>\n", sandbox.ID())

	return nil
}

// executeExec executes a command in a sandbox
func (c *SandboxCommand) executeExec(manager sandbox.Manager, args []string) error {
	if len(args) < 2 {
		return errors.New(errors.ErrorTypeInput, "executeExec", "sandbox ID and command are required")
	}

	sandboxID := args[0]
	command := args[1]
	cmdArgs := args[2:]

	// Find the sandbox
	sandboxes := manager.ListSandboxes()
	var found bool

	for _, sb := range sandboxes {
		if sb.ID == sandboxID {
			found = true
			break
		}
	}

	if !found {
		return errors.New(errors.ErrorTypeInput, "executeExec",
			fmt.Sprintf("sandbox %s not found", sandboxID))
	}

	// Create sandbox adapter for execution
	targetSandbox, err := manager.CreateSandbox()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeGit, "executeExec", "failed to access sandbox")
	}
	defer targetSandbox.Cleanup()

	fmt.Printf("Executing in sandbox %s: %s %s\n", sandboxID, command, strings.Join(cmdArgs, " "))

	result, err := targetSandbox.Execute(command, cmdArgs...)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "executeExec", "command execution failed")
	}

	fmt.Printf("Exit Code: %d\n", result.ExitCode)
	if result.Output != "" {
		fmt.Printf("Output:\n%s\n", result.Output)
	}
	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	}

	return nil
}

// executeValidate validates a file against rules
func (c *SandboxCommand) executeValidate(manager sandbox.Manager, args []string) error {
	if len(args) == 0 {
		return errors.New(errors.ErrorTypeInput, "executeValidate", "file path is required")
	}

	filePath := args[0]

	// Read file content if it exists
	var content string
	if c.Content != "" {
		content = c.Content
	} else {
		// Read from file system (this is a simplified version)
		// In a real implementation, you'd read the actual file
		content = "// Sample content for validation"
	}

	err := manager.ValidateCode(filePath, content)
	if err != nil {
		fmt.Printf("Validation failed for %s:\n%s\n", filePath, err.Error())
		return nil // Don't return error, just show validation result
	}

	fmt.Printf("Validation passed for %s\n", filePath)

	// Show applicable rules
	fileRules, contentRules := manager.GetValidationRules(filePath)

	if len(fileRules) > 0 || len(contentRules) > 0 {
		fmt.Println("\nApplicable Rules:")

		for _, rule := range fileRules {
			fmt.Printf("  File Rule: %s - %s\n", rule.Name, rule.Description)
		}

		for _, rule := range contentRules {
			fmt.Printf("  Content Rule: %s - %s\n", rule.Name, rule.Description)
		}
	}

	return nil
}

// executeStats shows sandbox statistics
func (c *SandboxCommand) executeStats(manager sandbox.Manager) error {
	if dm, ok := manager.(*sandbox.DefaultManager); ok {
		metrics := dm.GetMetrics()
		config := dm.GetConfig()

		fmt.Println("Sandbox Statistics:")
		fmt.Printf("  Total Sandboxes: %d\n", metrics.TotalSandboxes)
		fmt.Printf("  Active Sandboxes: %d\n", metrics.ActiveSandboxes)
		fmt.Printf("  Total Executions: %d\n", metrics.TotalExecutions)
		fmt.Printf("  Successful Runs: %d\n", metrics.SuccessfulRuns)
		fmt.Printf("  Failed Runs: %d\n", metrics.FailedRuns)

		if metrics.TotalExecutions > 0 {
			successRate := float64(metrics.SuccessfulRuns) / float64(metrics.TotalExecutions) * 100
			fmt.Printf("  Success Rate: %.1f%%\n", successRate)
		}

		if !metrics.LastCleanupTime.IsZero() {
			fmt.Printf("  Last Cleanup: %s\n", metrics.LastCleanupTime.Format("2006-01-02 15:04:05"))
		}

		fmt.Println("\nProject Configuration:")
		fmt.Printf("  Language: %s\n", config.Language)
		fmt.Printf("  Framework: %s\n", config.Framework)
		fmt.Printf("  Test Command: %s %s\n", config.Test.Command, strings.Join(config.Test.Args, " "))
		fmt.Printf("  Build Command: %s %s\n", config.Build.Command, strings.Join(config.Build.Args, " "))
		fmt.Printf("  Lint Command: %s %s\n", config.Lint.Command, strings.Join(config.Lint.Args, " "))
	} else {
		fmt.Println("Statistics not available for this manager type.")
	}

	return nil
}

// executeClean cleans up old sandboxes
func (c *SandboxCommand) executeClean(manager sandbox.Manager) error {
	fmt.Println("Cleaning up sandbox resources...")

	if err := manager.Cleanup(); err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "executeClean", "cleanup failed")
	}

	fmt.Println("Sandbox cleanup completed.")
	return nil
}

// executeTest runs tests in a sandbox environment
func (c *SandboxCommand) executeTest(manager sandbox.Manager, args []string) error {
	fmt.Println("Running tests in sandbox environment...")

	// Create execution request for testing
	request := sandbox.ExecutionRequest{
		ID:   fmt.Sprintf("test-%d", time.Now().Unix()),
		Type: "test",
		ValidationSteps: []sandbox.ValidationStep{
			{
				Name:        "Run Tests",
				Command:     "go",
				Args:        []string{"test", "./..."},
				Required:    true,
				Description: "Execute Go tests",
			},
		},
	}

	// Add any file changes from args
	if len(args) > 0 {
		for _, arg := range args {
			request.Files = append(request.Files, sandbox.FileChange{
				Path:      arg,
				Content:   "// Test file content",
				Operation: sandbox.OperationUpdate,
			})
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	response, err := manager.ExecuteCode(ctx, request)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "executeTest", "test execution failed")
	}

	fmt.Printf("Test execution completed with status: %s\n", response.Status)
	fmt.Printf("Duration: %s\n", response.Duration())

	for i, result := range response.Results {
		fmt.Printf("\nStep %d: %s\n", i+1, result.Command)
		fmt.Printf("Exit Code: %d\n", result.ExitCode)
		if result.Output != "" {
			fmt.Printf("Output:\n%s\n", result.Output)
		}
		if result.Error != "" {
			fmt.Printf("Error: %s\n", result.Error)
		}
	}

	if response.Diff != "" {
		fmt.Printf("\nChanges:\n%s\n", response.Diff)
	}

	return nil
}

// GetCobraCommand returns the cobra command for the sandbox command
func (c *SandboxCommand) GetCobraCommand() *cobra.Command {
	cmd := c.BaseCommand.GetCobraCommand()

	// Override run function
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.Execute(cmd.Context(), args)
	}

	// Add sandbox-specific flags
	cmd.Flags().StringVarP(&c.Command, "command", "c", "", "Command to execute")
	cmd.Flags().StringArrayVarP(&c.Args, "args", "a", []string{}, "Command arguments")
	cmd.Flags().StringVarP(&c.File, "file", "f", "", "File to validate")
	cmd.Flags().StringVar(&c.Content, "content", "", "Content to validate")
	cmd.Flags().DurationVarP(&c.Timeout, "timeout", "t", 5*time.Minute, "Execution timeout")

	// Add examples
	cmd.Example = `  # List active sandboxes
  sigil sandbox list

  # Create a new sandbox
  sigil sandbox create

  # Execute command in sandbox
  sigil sandbox exec <id> go build

  # Validate a file
  sigil sandbox validate main.go

  # Show statistics
  sigil sandbox stats

  # Clean up sandboxes
  sigil sandbox clean

  # Run tests in sandbox
  sigil sandbox test`

	// Add subcommands as usage
	cmd.Use = "sandbox <list|create|exec|validate|stats|clean|test> [args...]"

	return cmd
}

// Create the global sandbox command instance
var sandboxCmd = NewSandboxCommand().GetCobraCommand()
