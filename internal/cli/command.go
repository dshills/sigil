// Package cli provides command implementations for Sigil
package cli

import (
	"context"

	"github.com/dshills/sigil/internal/config"
	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
	"github.com/spf13/cobra"
)

// Command represents a Sigil CLI command
type Command interface {
	// Execute runs the command
	Execute(ctx context.Context, args []string) error

	// GetCobraCommand returns the cobra command for registration
	GetCobraCommand() *cobra.Command
}

// BaseCommand provides shared functionality for all commands
type BaseCommand struct {
	// Command metadata
	Name  string
	Short string
	Long  string

	// Common flags
	FileFlag   string
	DirFlag    string
	LinesFlag  string
	GitFlag    bool
	StagedFlag bool
	StdinFlag  bool

	// Output flags
	JSONFlag    bool
	PatchFlag   bool
	InPlaceFlag bool
	OutFlag     string

	// Model selection
	ModelFlag string

	// Context options
	IncludeMemoryFlag bool
	MemoryDepthFlag   int

	// Internal
	cmd *cobra.Command
}

// CommonFlags represents shared command flags
type CommonFlags struct {
	// Input sources
	File   string
	Dir    string
	Lines  string
	Git    bool
	Staged bool
	Stdin  bool

	// Output options
	JSON    bool
	Patch   bool
	InPlace bool
	Out     string

	// Model options
	Model string

	// Context options
	IncludeMemory bool
	MemoryDepth   int
}

// NewBaseCommand creates a new base command
func NewBaseCommand(name, short, long string) *BaseCommand {
	return &BaseCommand{
		Name:  name,
		Short: short,
		Long:  long,
	}
}

// GetCobraCommand returns the cobra command with common flags
func (b *BaseCommand) GetCobraCommand() *cobra.Command {
	if b.cmd == nil {
		b.cmd = &cobra.Command{
			Use:   b.Name,
			Short: b.Short,
			Long:  b.Long,
		}
		b.addCommonFlags()
	}
	return b.cmd
}

// addCommonFlags adds common flags to the command
func (b *BaseCommand) addCommonFlags() {
	// Input source flags
	b.cmd.Flags().StringVarP(&b.FileFlag, "file", "f", "", "Input file")
	b.cmd.Flags().StringVarP(&b.DirFlag, "dir", "d", "", "Input directory")
	b.cmd.Flags().StringVar(&b.LinesFlag, "lines", "", "Line range (e.g., 10-20)")
	b.cmd.Flags().BoolVar(&b.GitFlag, "git", false, "Use git diff as input")
	b.cmd.Flags().BoolVar(&b.StagedFlag, "staged", false, "Use staged git changes")
	b.cmd.Flags().BoolVar(&b.StdinFlag, "stdin", false, "Read from stdin")

	// Output flags
	b.cmd.Flags().BoolVar(&b.JSONFlag, "json", false, "Output in JSON format")
	b.cmd.Flags().BoolVar(&b.PatchFlag, "patch", false, "Output as patch")
	b.cmd.Flags().BoolVar(&b.InPlaceFlag, "in-place", false, "Modify files in place")
	b.cmd.Flags().StringVarP(&b.OutFlag, "out", "o", "", "Output file")

	// Model flags
	b.cmd.Flags().StringVarP(&b.ModelFlag, "model", "m", "", "Model to use (overrides config)")

	// Context flags
	b.cmd.Flags().BoolVar(&b.IncludeMemoryFlag, "include-memory", false, "Include memory context")
	b.cmd.Flags().IntVar(&b.MemoryDepthFlag, "memory-depth", 5, "Number of memory entries to include")
}

// GetCommonFlags extracts common flags into a struct
func (b *BaseCommand) GetCommonFlags() CommonFlags {
	return CommonFlags{
		File:          b.FileFlag,
		Dir:           b.DirFlag,
		Lines:         b.LinesFlag,
		Git:           b.GitFlag,
		Staged:        b.StagedFlag,
		Stdin:         b.StdinFlag,
		JSON:          b.JSONFlag,
		Patch:         b.PatchFlag,
		InPlace:       b.InPlaceFlag,
		Out:           b.OutFlag,
		Model:         b.ModelFlag,
		IncludeMemory: b.IncludeMemoryFlag,
		MemoryDepth:   b.MemoryDepthFlag,
	}
}

// ValidateFlags validates common flag combinations
func (b *BaseCommand) ValidateFlags() error {
	// Count input sources
	inputCount := 0
	if b.FileFlag != "" {
		inputCount++
	}
	if b.DirFlag != "" {
		inputCount++
	}
	if b.GitFlag {
		inputCount++
	}
	if b.StdinFlag {
		inputCount++
	}

	// Ensure only one input source
	if inputCount > 1 {
		return errors.ValidationError("ValidateFlags", "only one input source can be specified")
	}

	// Validate line range requires file
	if b.LinesFlag != "" && b.FileFlag == "" {
		return errors.ValidationError("ValidateFlags", "--lines requires --file")
	}

	// Validate staged requires git
	if b.StagedFlag && !b.GitFlag {
		return errors.ValidationError("ValidateFlags", "--staged requires --git")
	}

	// Validate output options
	outputCount := 0
	if b.JSONFlag {
		outputCount++
	}
	if b.PatchFlag {
		outputCount++
	}
	if b.InPlaceFlag {
		outputCount++
	}

	if outputCount > 1 {
		return errors.ValidationError("ValidateFlags", "only one output format can be specified")
	}

	// Validate in-place requires file
	if b.InPlaceFlag && b.FileFlag == "" {
		return errors.ValidationError("ValidateFlags", "--in-place requires --file")
	}

	return nil
}

// GetModel returns the model to use for the command
func (b *BaseCommand) GetModel(ctx context.Context) (model.Model, error) {
	modelStr := b.ModelFlag
	if modelStr == "" {
		// Use configured model
		cfg := getConfig()
		modelStr = cfg.Models.Lead
	}

	// Parse model string
	provider, modelName, err := model.ParseModelString(modelStr)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeModel, "GetModel", "invalid model string")
	}

	// Get or create model
	mdl, err := model.GetModel(provider, modelName)
	if err != nil {
		// Try to create it
		logger.Debug("model not cached, creating new instance", "provider", provider, "model", modelName)

		config := model.ModelConfig{
			Provider: provider,
			Model:    modelName,
		}

		// Get API key from config if available
		cfg := getConfig()
		if cfg.Models.Configs != nil {
			if providerCfg, ok := cfg.Models.Configs[provider]; ok {
				config.APIKey = providerCfg.APIKey
				config.Endpoint = providerCfg.Endpoint
				config.Options = providerCfg.Options
			}
		}

		mdl, err = model.CreateModel(config)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrorTypeModel, "GetModel", "failed to create model")
		}
	}

	return mdl, nil
}

// RunPreChecks performs common pre-execution checks
func (b *BaseCommand) RunPreChecks() error {
	// Validate flags
	if err := b.ValidateFlags(); err != nil {
		return err
	}

	// Additional checks can be added here

	return nil
}

// Helper function to get config
func getConfig() *config.Config {
	return config.Get()
}

// CommandContext holds context for command execution
type CommandContext struct {
	// Input data
	Input     string
	InputType InputType
	Files     []FileInput

	// Output options
	OutputFormat OutputFormat
	OutputPath   string

	// Model and context
	Model         model.Model
	MemoryContext []model.MemoryEntry
}

// InputType represents the type of input
type InputType string

const (
	InputTypeText      InputType = "text"
	InputTypeFile      InputType = "file"
	InputTypeDirectory InputType = "directory"
	InputTypeGitDiff   InputType = "git-diff"
)

// OutputFormat represents the output format
type OutputFormat string

const (
	OutputFormatText  OutputFormat = "text"
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatPatch OutputFormat = "patch"
)

// FileInput represents a file input
type FileInput struct {
	Path    string
	Content string
	Lines   []int // Specific lines if requested
}
