// Package config provides configuration management for Sigil.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
	"gopkg.in/yaml.v3"
)

// Config represents the complete Sigil configuration
type Config struct {
	// Model configuration
	Models ModelsConfig `yaml:"models"`

	// Sandbox configuration
	Sandbox SandboxConfig `yaml:"sandbox"`

	// Memory configuration
	Memory MemoryConfig `yaml:"memory"`

	// Rules configuration
	Rules []Rule `yaml:"rules"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging"`

	// Git configuration
	Git GitConfig `yaml:"git"`

	// Backend configuration (for MCP)
	Backend string     `yaml:"backend,omitempty"`
	MCP     *MCPConfig `yaml:"mcp,omitempty"`

	// Additional provider-specific configs
	Providers map[string]interface{} `yaml:"providers,omitempty"`
}

// ModelsConfig defines model configuration
type ModelsConfig struct {
	// Primary/lead model
	Lead string `yaml:"lead"`

	// Reviewer models for multi-agent workflows
	Reviewers []string `yaml:"reviewers,omitempty"`

	// Model-specific configurations
	Configs map[string]model.ModelConfig `yaml:"configs,omitempty"`
}

// SandboxConfig defines sandbox execution settings
type SandboxConfig struct {
	// Enable sandbox validation
	Enabled bool `yaml:"enabled"`

	// Timeout for sandbox operations
	Timeout time.Duration `yaml:"timeout"`

	// Cleanup temporary files
	Cleanup bool `yaml:"cleanup"`

	// Validation commands
	ValidationCommands []string `yaml:"validation_commands,omitempty"`
}

// MemoryConfig defines memory system settings
type MemoryConfig struct {
	// Enable memory system
	Enabled bool `yaml:"enabled"`

	// Maximum memory entries to include
	MaxEntries int `yaml:"max_entries"`

	// Memory file locations
	GlobalPath string `yaml:"global_path,omitempty"`
	LocalPath  string `yaml:"local_path,omitempty"`
}

// Rule represents a validation rule
type Rule struct {
	// Rule name
	Name string `yaml:"name"`

	// Command to execute
	Command string `yaml:"command"`

	// Must pass for commit
	MustPass bool `yaml:"must_pass"`

	// Timeout for rule execution
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

// LoggingConfig defines logging settings
type LoggingConfig struct {
	// Log level (debug, info, warn, error)
	Level string `yaml:"level"`

	// Log format (text, json)
	Format string `yaml:"format"`

	// Log file path (optional)
	File string `yaml:"file,omitempty"`
}

// GitConfig defines git-related settings
type GitConfig struct {
	// Auto-commit after successful operations
	AutoCommit bool `yaml:"auto_commit"`

	// Commit message template
	CommitTemplate string `yaml:"commit_template,omitempty"`

	// Create checkpoint commits
	Checkpoints bool `yaml:"checkpoints"`
}

// MCPConfig defines MCP server configuration
type MCPConfig struct {
	// Server URL
	ServerURL string `yaml:"server_url"`

	// Model name
	Model string `yaml:"model"`

	// API key (if required)
	APIKey string `yaml:"api_key,omitempty"`

	// Additional MCP-specific settings
	Settings map[string]interface{} `yaml:"settings,omitempty"`
}

// Default configuration values
var defaultConfig = Config{
	Models: ModelsConfig{
		Lead:    "openai:gpt-4",
		Configs: make(map[string]model.ModelConfig),
	},
	Sandbox: SandboxConfig{
		Enabled: true,
		Timeout: 5 * time.Minute,
		Cleanup: true,
	},
	Memory: MemoryConfig{
		Enabled:    true,
		MaxEntries: 10,
	},
	Logging: LoggingConfig{
		Level:  "info",
		Format: "text",
	},
	Git: GitConfig{
		AutoCommit:     false,
		Checkpoints:    true,
		CommitTemplate: "sigil: %s",
	},
}

// Global configuration instance
var (
	globalConfig *Config
	globalMu     sync.RWMutex
)

// Load loads configuration from file
func Load(path string) (*Config, error) {
	logger.Debug("loading configuration", "path", path)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("configuration file not found, using defaults", "path", path)
			return &defaultConfig, nil
		}
		return nil, errors.Wrap(err, errors.ErrorTypeConfig, "Load", "failed to open config file")
	}
	defer file.Close()

	config, err := Parse(file)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeConfig, "Load", "failed to parse config file")
	}

	// Apply environment variable overrides
	applyEnvOverrides(config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeConfig, "Load", "invalid configuration")
	}

	// Set global config with thread safety
	globalMu.Lock()
	globalConfig = config
	globalMu.Unlock()

	return config, nil
}

// Parse parses configuration from reader
func Parse(r io.Reader) (*Config, error) {
	config := defaultConfig // Start with defaults

	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	return &config, nil
}

// Save saves configuration to file
func (c *Config) Save(path string) error {
	logger.Debug("saving configuration", "path", path)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "Save", "failed to create directory")
	}

	file, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "Save", "failed to create file")
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	defer encoder.Close()

	if err := encoder.Encode(c); err != nil {
		return errors.Wrap(err, errors.ErrorTypeConfig, "Save", "failed to encode YAML")
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate lead model
	if c.Models.Lead == "" {
		return errors.ConfigError("Validate", "lead model is required")
	}

	// Validate model format
	if _, _, err := model.ParseModelString(c.Models.Lead); err != nil {
		return errors.ConfigError("Validate", fmt.Sprintf("invalid lead model format: %s", c.Models.Lead))
	}

	// Validate reviewers
	for _, reviewer := range c.Models.Reviewers {
		if _, _, err := model.ParseModelString(reviewer); err != nil {
			return errors.ConfigError("Validate", fmt.Sprintf("invalid reviewer model format: %s", reviewer))
		}
	}

	// Validate logging level
	validLevels := []string{"debug", "info", "warn", "error"}
	isValidLevel := false
	for _, level := range validLevels {
		if strings.ToLower(c.Logging.Level) == level {
			isValidLevel = true
			break
		}
	}
	if !isValidLevel {
		return errors.ConfigError("Validate", fmt.Sprintf("invalid log level: %s", c.Logging.Level))
	}

	// Validate MCP config if backend is MCP
	if strings.ToLower(c.Backend) == "mcp" && c.MCP == nil {
		return errors.ConfigError("Validate", "MCP configuration required when backend is 'mcp'")
	}

	return nil
}

// Get returns the global configuration
func Get() *Config {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalConfig == nil {
		return &defaultConfig
	}
	return globalConfig
}

// Set sets the global configuration
func Set(config *Config) {
	globalMu.Lock()
	defer globalMu.Unlock()

	globalConfig = config
}

// applyEnvOverrides applies environment variable overrides
func applyEnvOverrides(config *Config) {
	// Override model from environment
	if lead := os.Getenv("SIGIL_MODEL"); lead != "" {
		config.Models.Lead = lead
	}

	// Override log level
	if level := os.Getenv("SIGIL_LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}

	// Override API keys for providers
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		ensureModelConfig(config, "openai")
		cfg := config.Models.Configs["openai"]
		cfg.APIKey = apiKey
		config.Models.Configs["openai"] = cfg
	}
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		ensureModelConfig(config, "anthropic")
		cfg := config.Models.Configs["anthropic"]
		cfg.APIKey = apiKey
		config.Models.Configs["anthropic"] = cfg
	}
}

// ensureModelConfig ensures a model config exists for a provider
func ensureModelConfig(config *Config, provider string) {
	if config.Models.Configs == nil {
		config.Models.Configs = make(map[string]model.ModelConfig)
	}

	if _, exists := config.Models.Configs[provider]; !exists {
		config.Models.Configs[provider] = model.ModelConfig{Provider: provider}
	}
}
