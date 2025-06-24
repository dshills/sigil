package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/memory"
	"github.com/spf13/cobra"
)

// MemoryCommand implements the memory command
type MemoryCommand struct {
	*BaseCommand
	Subcommand string
	Query      string
	Limit      int
	Format     string
	Output     string
}

// NewMemoryCommand creates a new memory command
func NewMemoryCommand() *MemoryCommand {
	return &MemoryCommand{
		BaseCommand: NewBaseCommand(
			"memory",
			"Manage memory entries",
			`The memory command allows you to view, search, and manage the stored memory entries
that Sigil uses for context and history.

Examples:
  sigil memory list                      # List recent memory entries
  sigil memory search "error handling"   # Search memory for specific content
  sigil memory stats                     # Show memory statistics
  sigil memory clean                     # Clean old memory entries`,
		),
		Limit:  10,
		Format: "text",
	}
}

// Execute runs the memory command
func (c *MemoryCommand) Execute(ctx context.Context, args []string) error {
	if err := c.RunPreChecks(); err != nil {
		return err
	}

	// Determine subcommand
	if len(args) > 0 {
		c.Subcommand = args[0]
	} else {
		c.Subcommand = "list" // Default
	}

	// Create memory manager
	memManager, err := memory.NewManager()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeConfig, "Execute", "failed to create memory manager")
	}

	logger.Debug("executing memory command", "subcommand", c.Subcommand)

	switch c.Subcommand {
	case "list":
		return c.executeList(memManager)
	case "search":
		return c.executeSearch(memManager, args[1:])
	case "stats":
		return c.executeStats(memManager)
	case "clean":
		return c.executeClean(memManager)
	case "export":
		return c.executeExport(memManager, args[1:])
	default:
		return errors.New(errors.ErrorTypeInput, "Execute",
			fmt.Sprintf("unknown memory subcommand: %s", c.Subcommand))
	}
}

// executeList lists recent memory entries
func (c *MemoryCommand) executeList(memManager memory.Manager) error {
	entries, err := memManager.GetRecentSessions(c.Limit)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "executeList", "failed to get recent sessions")
	}

	if len(entries) == 0 {
		fmt.Println("No memory entries found.")
		return nil
	}

	fmt.Printf("Recent Memory Entries (%d):\n\n", len(entries))

	for i, entry := range entries {
		fmt.Printf("%d. [%s] %s\n", i+1, entry.Type, entry.Summary)
		fmt.Printf("   Time: %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"))
		if entry.Command != "" {
			fmt.Printf("   Command: %s\n", entry.Command)
		}
		if entry.Model != "" {
			fmt.Printf("   Model: %s\n", entry.Model)
		}
		if entry.TokensUsed > 0 {
			fmt.Printf("   Tokens: %d\n", entry.TokensUsed)
		}

		// Show preview of content (first 100 chars)
		preview := entry.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		fmt.Printf("   Content: %s\n", strings.ReplaceAll(preview, "\n", " "))
		fmt.Println()
	}

	return nil
}

// executeSearch searches memory entries
func (c *MemoryCommand) executeSearch(memManager memory.Manager, args []string) error {
	if len(args) == 0 {
		return errors.New(errors.ErrorTypeInput, "executeSearch", "search query is required")
	}

	query := strings.Join(args, " ")
	entries, err := memManager.SearchMemory(query, c.Limit)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "executeSearch", "failed to search memory")
	}

	if len(entries) == 0 {
		fmt.Printf("No memory entries found matching '%s'.\n", query)
		return nil
	}

	fmt.Printf("Search Results for '%s' (%d):\n\n", query, len(entries))

	for i, entry := range entries {
		fmt.Printf("%d. [%s] %s\n", i+1, entry.Type, entry.Summary)
		fmt.Printf("   Time: %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"))
		if entry.Command != "" {
			fmt.Printf("   Command: %s\n", entry.Command)
		}

		// Show preview with highlighted query
		preview := entry.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		fmt.Printf("   Content: %s\n", strings.ReplaceAll(preview, "\n", " "))
		fmt.Println()
	}

	return nil
}

// executeStats shows memory statistics
func (c *MemoryCommand) executeStats(memManager memory.Manager) error {
	stats, err := memManager.GetStats()
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "executeStats", "failed to get memory stats")
	}

	fmt.Println("Memory Statistics:")
	fmt.Printf("  Total Entries: %d\n", stats.TotalEntries)

	if stats.TotalEntries > 0 {
		fmt.Printf("  Total Size: %d bytes (%.2f KB)\n", stats.TotalSizeBytes, float64(stats.TotalSizeBytes)/1024)
		fmt.Printf("  Average Size: %d bytes\n", stats.AverageSize)
		fmt.Printf("  Oldest Entry: %s\n", stats.OldestEntry.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Newest Entry: %s\n", stats.NewestEntry.Format("2006-01-02 15:04:05"))

		fmt.Println("\n  Entries by Type:")
		for entryType, count := range stats.EntriesByType {
			fmt.Printf("    %s: %d\n", entryType, count)
		}
	}

	fmt.Printf("\n  Memory Directory: %s\n", memory.GetMemoryDirectory())

	return nil
}

// executeClean cleans old memory entries
func (c *MemoryCommand) executeClean(memManager memory.Manager) error {
	// Default retention: 30 days
	retention := 30 * 24 * time.Hour

	if err := memManager.CleanOldEntries(retention); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "executeClean", "failed to clean old entries")
	}

	fmt.Printf("Cleaned memory entries older than %d days.\n", 30)
	return nil
}

// executeExport exports memory entries
func (c *MemoryCommand) executeExport(memManager memory.Manager, args []string) error {
	if len(args) == 0 {
		return errors.New(errors.ErrorTypeInput, "executeExport", "export filename is required")
	}

	filename := args[0]
	format := c.Format

	// Try to infer format from filename
	if strings.HasSuffix(filename, ".json") {
		format = "json"
	} else if strings.HasSuffix(filename, ".md") || strings.HasSuffix(filename, ".markdown") {
		format = "markdown"
	}

	if dm, ok := memManager.(*memory.DefaultManager); ok {
		if err := dm.ExportMemory(format, filename); err != nil {
			return errors.Wrap(err, errors.ErrorTypeFS, "executeExport", "failed to export memory")
		}
	} else {
		return errors.New(errors.ErrorTypeInternal, "executeExport", "export not supported by this memory manager")
	}

	fmt.Printf("Memory exported to %s (format: %s)\n", filename, format)
	return nil
}

// GetCobraCommand returns the cobra command for the memory command
func (c *MemoryCommand) GetCobraCommand() *cobra.Command {
	cmd := c.BaseCommand.GetCobraCommand()

	// Override run function
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.Execute(cmd.Context(), args)
	}

	// Add memory-specific flags
	cmd.Flags().IntVarP(&c.Limit, "limit", "l", 10, "Limit number of results")
	cmd.Flags().StringVarP(&c.Format, "format", "f", "text", "Export format (text, markdown, json)")
	cmd.Flags().StringVarP(&c.Output, "output", "o", "", "Output file for export")

	// Add examples
	cmd.Example = `  # List recent memory entries
  sigil memory list

  # Search for specific content
  sigil memory search "error handling"

  # Show memory statistics
  sigil memory stats

  # Clean old memory entries
  sigil memory clean

  # Export memory to file
  sigil memory export memory_backup.md`

	// Add subcommands as usage
	cmd.Use = "memory <list|search|stats|clean|export> [args...]"

	return cmd
}

// Create the global memory command instance
var memoryCmd = NewMemoryCommand().GetCobraCommand()
