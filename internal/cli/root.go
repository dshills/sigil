package cli

import (
	"fmt"
	"os"

	"github.com/dshills/sigil/internal/config"
	"github.com/dshills/sigil/internal/git"
	"github.com/dshills/sigil/internal/model"
	"github.com/dshills/sigil/internal/model/providers/anthropic"
	"github.com/dshills/sigil/internal/model/providers/mcp"
	"github.com/dshills/sigil/internal/model/providers/ollama"
	"github.com/dshills/sigil/internal/model/providers/openai"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	verboseFlag bool
	jsonFlag    bool
	configFile  string

	// Root command
	rootCmd = &cobra.Command{
		Use:   "sigil",
		Short: "Intelligent code transformation using LLMs",
		Long: `Sigil is a command-line tool that enables intelligent, autonomous code
transformation, explanation, review, and generation using local or remote LLMs.

It supports multiple LLM backends, sandboxed validation, fully autonomous execution,
memory persistence via Markdown files, and integration with MCP servers.`,
		Version: "0.1.0",
	}
)

// Execute runs the CLI
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file (default: .sigil/config.yml)")

	// Add commands
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(explainCmd)
	rootCmd.AddCommand(summarizeCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(docCmd)
}

func initConfig() {
	// Check if we're in a Git repository
	if err := checkGitRepository(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	if _, err := config.Load(configFile); err != nil {
		if verboseFlag {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load config: %v\n", err)
		}
		// Continue with default configuration
	}

	// Register model providers
	initModelProviders()
}

func initModelProviders() {
	// Register all providers
	providers := map[string]model.Factory{
		"openai":    openai.NewProvider(),
		"anthropic": anthropic.NewProvider(),
		"ollama":    ollama.NewProvider(),
		"mcp":       mcp.NewProvider(),
	}

	for name, provider := range providers {
		if err := model.RegisterProvider(name, provider); err != nil {
			if verboseFlag {
				fmt.Fprintf(os.Stderr, "Warning: Failed to register provider %s: %v\n", name, err)
			}
		}
	}
}

func checkGitRepository() error {
	if err := git.IsGitRepository(); err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	return nil
}
