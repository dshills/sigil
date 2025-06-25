package mcp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dshills/sigil/internal/config"
	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"gopkg.in/yaml.v3"
)

// ConfigLoader handles loading MCP server configurations
type ConfigLoader struct {
	globalPath  string
	projectPath string
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(globalPath, projectPath string) *ConfigLoader {
	return &ConfigLoader{
		globalPath:  globalPath,
		projectPath: projectPath,
	}
}

// LoadConfigurations loads MCP server configurations from various sources
func (cl *ConfigLoader) LoadConfigurations() ([]ServerConfig, error) {
	configs := make([]ServerConfig, 0)
	configMap := make(map[string]ServerConfig)

	// Load from main config if available
	mainConfig := config.Get()
	if mainConfig.MCP != nil && len(mainConfig.MCP.Servers) > 0 {
		for _, srv := range mainConfig.MCP.Servers {
			serverConfig := ServerConfig{
				Name:        srv.Name,
				Command:     srv.Command,
				Args:        srv.Args,
				Env:         srv.Env,
				Transport:   srv.Transport,
				WorkingDir:  srv.WorkingDir,
				AutoRestart: srv.AutoRestart,
				MaxRestarts: srv.MaxRestarts,
				Settings: struct {
					Timeout    string `yaml:"timeout" json:"timeout"`
					MaxRetries int    `yaml:"maxRetries" json:"maxRetries"`
				}{
					Timeout:    srv.Settings.Timeout,
					MaxRetries: srv.Settings.MaxRetries,
				},
			}
			configMap[srv.Name] = serverConfig
		}
	}

	// Load from global MCP servers file
	if cl.globalPath != "" {
		globalConfigs, err := cl.loadFromFile(cl.globalPath)
		if err != nil && !os.IsNotExist(err) {
			logger.Warn("failed to load global MCP configs", "error", err)
		} else {
			for _, cfg := range globalConfigs {
				configMap[cfg.Name] = cfg
			}
		}
	}

	// Load from project-specific MCP servers file
	if cl.projectPath != "" {
		projectConfigs, err := cl.loadFromFile(cl.projectPath)
		if err != nil && !os.IsNotExist(err) {
			logger.Warn("failed to load project MCP configs", "error", err)
		} else {
			for _, cfg := range projectConfigs {
				configMap[cfg.Name] = cfg // Project configs override global
			}
		}
	}

	// Convert map to slice
	for _, cfg := range configMap {
		configs = append(configs, cfg)
	}

	return configs, nil
}

// loadFromFile loads MCP server configurations from a YAML file
func (cl *ConfigLoader) loadFromFile(path string) ([]ServerConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return cl.parseFile(file)
}

// parseFile parses MCP server configurations from a reader
func (cl *ConfigLoader) parseFile(r io.Reader) ([]ServerConfig, error) {
	var fileConfig struct {
		Servers []ServerConfig `yaml:"servers"`
	}

	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&fileConfig); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	// Validate and set defaults
	for i := range fileConfig.Servers {
		srv := &fileConfig.Servers[i]

		if srv.Name == "" {
			return nil, errors.ConfigError("parseFile", "server name is required")
		}

		if srv.Command == "" {
			return nil, errors.ConfigError("parseFile", fmt.Sprintf("command is required for server %s", srv.Name))
		}

		// Set defaults
		if srv.Transport == "" {
			srv.Transport = "stdio"
		}

		if srv.MaxRestarts == 0 && srv.AutoRestart {
			srv.MaxRestarts = 3
		}
	}

	return fileConfig.Servers, nil
}

// GetDefaultPaths returns the default paths for MCP configuration files
func GetDefaultPaths() (globalPath, projectPath string) {
	// Global path: ~/.config/sigil/mcp-servers.yml
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalPath = filepath.Join(homeDir, ".config", "sigil", "mcp-servers.yml")
	}

	// Project path: .sigil/mcp-servers.yml (relative to current directory)
	projectPath = filepath.Join(".sigil", "mcp-servers.yml")

	return globalPath, projectPath
}

// SaveExample saves an example MCP servers configuration
func SaveExample(path string) error {
	example := `# MCP Server Configuration
# This file defines Model Context Protocol servers that Sigil can use

servers:
  # GitHub MCP Server
  - name: github-mcp
    command: npx
    args: [-y, "@modelcontextprotocol/server-github"]
    env:
      GITHUB_TOKEN: ${GITHUB_TOKEN}
    transport: stdio
    auto_restart: true
    max_restarts: 3
    settings:
      timeout: 30s
      max_retries: 3

  # PostgreSQL MCP Server
  - name: postgres-mcp
    command: mcp-server-postgres
    args: ["--database", "mydb"]
    env:
      DATABASE_URL: ${DATABASE_URL}
    transport: stdio
    auto_restart: true

  # Custom Python MCP Server
  - name: custom-python
    command: python
    args: ["-m", "my_mcp_server"]
    working_dir: /path/to/server
    transport: stdio
    settings:
      timeout: 60s

  # WebSocket-based Server
  - name: websocket-server
    command: ws-mcp-server
    args: ["--port", "8080"]
    transport: websocket
    auto_restart: false
`

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write example file
	if err := os.WriteFile(path, []byte(example), 0600); err != nil {
		return fmt.Errorf("failed to write example file: %w", err)
	}

	return nil
}
