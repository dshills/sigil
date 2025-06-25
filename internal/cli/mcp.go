// Package cli provides command-line interface implementations for Sigil.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"time"

	"github.com/dshills/sigil/internal/model"
	"github.com/dshills/sigil/internal/model/providers/mcp"
	"github.com/spf13/cobra"
)

// NewMCPCommand creates the MCP management command
func NewMCPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP (Model Context Protocol) servers",
		Long: `Manage MCP servers including configuration, starting, stopping, and listing servers.

MCP servers extend Sigil's capabilities by providing access to tools, resources,
and external models through the Model Context Protocol.`,
		Example: `  # List configured MCP servers
  sigil mcp list

  # Start a specific MCP server
  sigil mcp start github-mcp

  # Stop a running MCP server
  sigil mcp stop github-mcp

  # Show status of all servers
  sigil mcp status

  # Generate example configuration
  sigil mcp init`,
	}

	// Add subcommands
	cmd.AddCommand(newMCPListCommand())
	cmd.AddCommand(newMCPStartCommand())
	cmd.AddCommand(newMCPStopCommand())
	cmd.AddCommand(newMCPStatusCommand())
	cmd.AddCommand(newMCPInitCommand())

	return cmd
}

// newMCPListCommand creates the list subcommand
func newMCPListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured MCP servers",
		Long:  "Display all MCP servers defined in configuration files.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configurations
			globalPath, projectPath := mcp.GetDefaultPaths()
			loader := mcp.NewConfigLoader(globalPath, projectPath)

			configs, err := loader.LoadConfigurations()
			if err != nil {
				return fmt.Errorf("failed to load configurations: %w", err)
			}

			if len(configs) == 0 {
				fmt.Println("No MCP servers configured.")
				fmt.Println("\nRun 'sigil mcp init' to create an example configuration.")
				return nil
			}

			// Display servers
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tCOMMAND\tTRANSPORT\tAUTO-RESTART")
			fmt.Fprintln(w, "----\t-------\t---------\t------------")

			for _, cfg := range configs {
				fmt.Fprintf(w, "%s\t%s\t%s\t%v\n",
					cfg.Name,
					cfg.Command,
					cfg.Transport,
					cfg.AutoRestart)
			}

			w.Flush()
			return nil
		},
	}
}

// newMCPStartCommand creates the start subcommand
func newMCPStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start <server-name>",
		Short: "Start an MCP server",
		Long:  "Start a configured MCP server by name.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverName := args[0]

			// Create provider
			provider := mcp.NewProvider()
			defer provider.Shutdown()

			// Create a model to trigger server start
			config := model.ModelConfig{
				Endpoint: fmt.Sprintf("mcp://%s", serverName),
			}

			_, err := provider.CreateModel(config)
			if err != nil {
				return fmt.Errorf("failed to start server: %w", err)
			}

			fmt.Printf("Started MCP server: %s\n", serverName)
			return nil
		},
	}
}

// newMCPStopCommand creates the stop subcommand
func newMCPStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <server-name>",
		Short: "Stop a running MCP server",
		Long:  "Stop a running MCP server by name.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverName := args[0]

			// Create process manager
			pm := mcp.NewProcessManager()

			if err := pm.StopServer(serverName); err != nil {
				return fmt.Errorf("failed to stop server: %w", err)
			}

			fmt.Printf("Stopped MCP server: %s\n", serverName)
			return nil
		},
	}
}

// newMCPStatusCommand creates the status subcommand
func newMCPStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of MCP servers",
		Long:  "Display the current status of all managed MCP servers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create process manager
			pm := mcp.NewProcessManager()
			servers := pm.ListServers()

			if len(servers) == 0 {
				fmt.Println("No MCP servers are currently running.")
				return nil
			}

			// Display status
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSTATUS\tUPTIME\tLAST ERROR")
			fmt.Fprintln(w, "----\t------\t------\t----------")

			for _, server := range servers {
				connected, uptime, lastErr := server.GetStatus()
				status := "disconnected"
				if connected {
					status = "connected"
				}

				errMsg := "-"
				if lastErr != nil {
					errMsg = lastErr.Error()
					if len(errMsg) > 40 {
						errMsg = errMsg[:40] + "..."
					}
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					server.Name,
					status,
					formatDuration(uptime),
					errMsg)
			}

			w.Flush()
			return nil
		},
	}
}

// newMCPInitCommand creates the init subcommand
func newMCPInitCommand() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create example MCP server configuration",
		Long: `Create an example MCP server configuration file.

By default, creates a project-specific configuration in .sigil/mcp-servers.yml.
Use --global to create a user-wide configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var path string

			if global {
				// Global configuration
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				path = filepath.Join(homeDir, ".config", "sigil", "mcp-servers.yml")
			} else {
				// Project configuration
				path = filepath.Join(".sigil", "mcp-servers.yml")
			}

			// Check if file already exists
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("configuration file already exists: %s", path)
			}

			// Save example
			if err := mcp.SaveExample(path); err != nil {
				return fmt.Errorf("failed to create configuration: %w", err)
			}

			fmt.Printf("Created example MCP configuration: %s\n", path)
			fmt.Println("\nEdit this file to configure your MCP servers.")
			fmt.Println("Then run 'sigil mcp list' to verify the configuration.")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Create global configuration instead of project-specific")

	return cmd
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}
