package config

import (
	"github.com/dshills/sigil/internal/model"
)

// Config represents the complete configuration for Sigil
type Config struct {
	// Model configuration
	Models ModelsConfig `yaml:"models"`

	// Backend configuration
	Backend string `yaml:"backend"`

	// MCP configuration
	MCP *MCPConfig `yaml:"mcp,omitempty"`

	// Rules configuration
	Rules []Rule `yaml:"rules,omitempty"`

	// Memory configuration
	Memory MemoryConfig `yaml:"memory"`

	// Sandbox configuration
	Sandbox SandboxConfig `yaml:"sandbox"`
}

// ModelsConfig represents model configuration
type ModelsConfig struct {
	Lead      string   `yaml:"lead"`
	Reviewers []string `yaml:"reviewers,omitempty"`
}

// MCPConfig represents MCP server configuration
type MCPConfig struct {
	ServerURL string `yaml:"server_url"`
	Model     string `yaml:"model"`
}

// Rule represents a validation rule
type Rule struct {
	MustPass    string `yaml:"must_pass,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// MemoryConfig represents memory system configuration
type MemoryConfig struct {
	Enabled    bool   `yaml:"enabled"`
	GlobalPath string `yaml:"global_path,omitempty"`
	LocalPath  string `yaml:"local_path,omitempty"`
	MaxDepth   int    `yaml:"max_depth"`
	MaxEntries int    `yaml:"max_entries"`
}

// SandboxConfig represents sandbox configuration
type SandboxConfig struct {
	Enabled     bool     `yaml:"enabled"`
	Timeout     int      `yaml:"timeout"` // seconds
	MaxAttempts int      `yaml:"max_attempts"`
	Validators  []string `yaml:"validators,omitempty"`
}

// Default returns a default configuration
func Default() *Config {
	return &Config{
		Models: ModelsConfig{
			Lead: "openai:gpt-4",
		},
		Backend: "openai",
		Memory: MemoryConfig{
			Enabled:    true,
			GlobalPath: "~/.sigil/SIGIL.md",
			LocalPath:  ".sigil/SIGIL.md",
			MaxDepth:   10,
			MaxEntries: 100,
		},
		Sandbox: SandboxConfig{
			Enabled:     true,
			Timeout:     300, // 5 minutes
			MaxAttempts: 3,
		},
		Rules: []Rule{
			{
				MustPass:    "go test ./...",
				Description: "All tests must pass",
			},
			{
				MustPass:    "golangci-lint run",
				Description: "Code must pass linting",
			},
		},
	}
}

// ToModelConfig converts a model string to ModelConfig
func (c *Config) ToModelConfig(modelStr string) model.ModelConfig {
	// Parse model string format: "provider:model"
	parts := splitModelString(modelStr)
	provider := parts[0]
	modelName := ""
	if len(parts) > 1 {
		modelName = parts[1]
	}

	config := model.ModelConfig{
		Provider: provider,
		Model:    modelName,
		Options:  make(map[string]interface{}),
	}

	// Add MCP-specific configuration if applicable
	if provider == "mcp" && c.MCP != nil {
		config.Endpoint = c.MCP.ServerURL
		if modelName == "" {
			config.Model = c.MCP.Model
		}
	}

	return config
}

// splitModelString splits a model string like "openai:gpt-4" into parts
func splitModelString(modelStr string) []string {
	// Simple split on colon
	var parts []string
	colonIndex := -1

	for i, ch := range modelStr {
		if ch == ':' {
			colonIndex = i
			break
		}
	}

	if colonIndex == -1 {
		parts = []string{modelStr}
	} else {
		parts = []string{modelStr[:colonIndex], modelStr[colonIndex+1:]}
	}

	return parts
}
