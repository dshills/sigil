package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Loader handles configuration loading
type Loader struct {
	searchPaths []string
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		searchPaths: []string{
			".sigil/config.yml",
			".sigil/config.yaml",
			"sigil.yml",
			"sigil.yaml",
		},
	}
}

// Load loads configuration from file or returns default
func (l *Loader) Load(configFile string) (*Config, error) {
	// If specific config file is provided, use it
	if configFile != "" {
		return l.loadFromFile(configFile)
	}

	// Search for config file in default locations
	for _, path := range l.searchPaths {
		if _, err := os.Stat(path); err == nil {
			config, err := l.loadFromFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
			}
			return config, nil
		}
	}

	// Check home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalConfig := filepath.Join(homeDir, ".sigil", "config.yml")
		if _, err := os.Stat(globalConfig); err == nil {
			config, err := l.loadFromFile(globalConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to load global config: %w", err)
			}
			return config, nil
		}
	}

	// Return default configuration
	return &defaultConfig, nil
}

// loadFromFile loads configuration from a specific file
func (l *Loader) loadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Start with default config
	config := defaultConfig

	// Unmarshal YAML into config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := l.validate(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Expand paths
	l.expandPaths(&config)

	return &config, nil
}

// validate checks if the configuration is valid
func (l *Loader) validate(config *Config) error {
	// Validate model configuration
	if config.Models.Lead == "" {
		return fmt.Errorf("lead model must be specified")
	}

	// Validate backend if specified
	if config.Backend != "" {
		validBackends := map[string]bool{
			"openai":    true,
			"anthropic": true,
			"ollama":    true,
			"mcp":       true,
		}

		if !validBackends[config.Backend] {
			return fmt.Errorf("invalid backend: %s", config.Backend)
		}
	}

	// Validate MCP configuration if backend is MCP
	if config.Backend == "mcp" {
		if config.MCP == nil {
			return fmt.Errorf("MCP configuration required when backend is 'mcp'")
		}
		if config.MCP.ServerURL == "" {
			return fmt.Errorf("MCP server URL must be specified")
		}
	}

	// Validate memory configuration
	if config.Memory.MaxEntries <= 0 {
		config.Memory.MaxEntries = 10
	}

	// Validate sandbox configuration
	if config.Sandbox.Timeout <= 0 {
		config.Sandbox.Timeout = 5 * time.Minute
	}

	return nil
}

// expandPaths expands paths in the configuration
func (l *Loader) expandPaths(config *Config) {
	// Expand home directory in paths
	if config.Memory.GlobalPath != "" {
		config.Memory.GlobalPath = expandPath(config.Memory.GlobalPath)
	}
	if config.Memory.LocalPath != "" {
		config.Memory.LocalPath = expandPath(config.Memory.LocalPath)
	}
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(homeDir, path[1:])
		}
	}
	return path
}

// Save saves the configuration to a file
func (l *Loader) Save(config *Config, path string) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal configuration to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// CreateDefaultConfig creates a default configuration file
func CreateDefaultConfig() error {
	loader := NewLoader()
	config := &defaultConfig

	// Try to create in .sigil directory
	return loader.Save(config, ".sigil/config.yml")
}
