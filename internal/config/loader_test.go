package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoader(t *testing.T) {
	loader := NewLoader()
	assert.NotNil(t, loader)
	assert.NotEmpty(t, loader.searchPaths)
	assert.Contains(t, loader.searchPaths, ".sigil/config.yml")
	assert.Contains(t, loader.searchPaths, ".sigil/config.yaml")
	assert.Contains(t, loader.searchPaths, "sigil.yml")
	assert.Contains(t, loader.searchPaths, "sigil.yaml")
}

func TestLoaderLoad(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()

	// Change to temporary directory for testing
	err := os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	loader := NewLoader()

	t.Run("load specific config file", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "specific.yml")
		configContent := `
models:
  lead: "anthropic:claude-3"
logging:
  level: "debug"
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		config, err := loader.Load(configPath)
		require.NoError(t, err)
		assert.Equal(t, "anthropic:claude-3", config.Models.Lead)
		assert.Equal(t, "debug", config.Logging.Level)
	})

	t.Run("load from search paths", func(t *testing.T) {
		// Create .sigil directory and config
		err := os.MkdirAll(".sigil", 0755)
		require.NoError(t, err)

		configContent := `
models:
  lead: "openai:gpt-3.5"
logging:
  level: "warn"
`
		configPath := ".sigil/config.yml"
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		config, err := loader.Load("")
		require.NoError(t, err)
		assert.Equal(t, "openai:gpt-3.5", config.Models.Lead)
		assert.Equal(t, "warn", config.Logging.Level)

		// Clean up
		os.RemoveAll(".sigil")
	})

	t.Run("load from sigil.yml in current directory", func(t *testing.T) {
		configContent := `
models:
  lead: "ollama:llama2"
logging:
  level: "error"
`
		configPath := "sigil.yml"
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		config, err := loader.Load("")
		require.NoError(t, err)
		assert.Equal(t, "ollama:llama2", config.Models.Lead)
		assert.Equal(t, "error", config.Logging.Level)

		// Clean up
		os.Remove(configPath)
	})

	t.Run("load defaults when no config found", func(t *testing.T) {
		// Ensure no config files exist
		for _, path := range loader.searchPaths {
			os.Remove(path)
		}

		config, err := loader.Load("")
		require.NoError(t, err)
		// Should return default config
		assert.Equal(t, "openai:gpt-4", config.Models.Lead)
		assert.Equal(t, "info", config.Logging.Level)
	})

	t.Run("error on invalid specific file", func(t *testing.T) {
		invalidPath := filepath.Join(tmpDir, "invalid.yml")
		invalidContent := `invalid: yaml: content: [`
		err := os.WriteFile(invalidPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		config, err := loader.Load(invalidPath)
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("error on non-existent specific file", func(t *testing.T) {
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.yml")
		config, err := loader.Load(nonExistentPath)
		assert.Error(t, err)
		assert.Nil(t, config)
	})
}

func TestLoaderLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader()

	t.Run("load valid config file", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "valid.yml")
		configContent := `
models:
  lead: "anthropic:claude-3"
  reviewers:
    - "openai:gpt-4"
backend: "anthropic"
memory:
  enabled: true
  max_entries: 20
  global_path: "~/global_memory"
  local_path: "./local_memory"
sandbox:
  enabled: false
  timeout: 10m
  cleanup: false
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		config, err := loader.loadFromFile(configPath)
		require.NoError(t, err)
		assert.Equal(t, "anthropic:claude-3", config.Models.Lead)
		assert.Equal(t, []string{"openai:gpt-4"}, config.Models.Reviewers)
		assert.Equal(t, "anthropic", config.Backend)
		assert.True(t, config.Memory.Enabled)
		assert.Equal(t, 20, config.Memory.MaxEntries)
		assert.False(t, config.Sandbox.Enabled)
		assert.Equal(t, 10*time.Minute, config.Sandbox.Timeout)
	})

	t.Run("error on invalid YAML", func(t *testing.T) {
		invalidPath := filepath.Join(tmpDir, "invalid.yml")
		invalidContent := `models: {lead: "test"} invalid: [`
		err := os.WriteFile(invalidPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		config, err := loader.loadFromFile(invalidPath)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "failed to parse config file")
	})

	t.Run("error on validation failure", func(t *testing.T) {
		invalidModelPath := filepath.Join(tmpDir, "invalid_model.yml")
		invalidModelContent := `
models:
  lead: ""
`
		err := os.WriteFile(invalidModelPath, []byte(invalidModelContent), 0644)
		require.NoError(t, err)

		config, err := loader.loadFromFile(invalidModelPath)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "invalid configuration")
	})
}

func TestLoaderValidate(t *testing.T) {
	loader := NewLoader()

	t.Run("valid config passes", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
			},
			Memory: MemoryConfig{
				MaxEntries: 10,
			},
			Sandbox: SandboxConfig{
				Timeout: 5 * time.Minute,
			},
		}

		err := loader.validate(config)
		assert.NoError(t, err)
	})

	t.Run("empty lead model fails", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "",
			},
		}

		err := loader.validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lead model must be specified")
	})

	t.Run("invalid backend fails", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
			},
			Backend: "invalid-backend",
		}

		err := loader.validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid backend")
	})

	t.Run("valid backends pass", func(t *testing.T) {
		validBackends := []string{"openai", "anthropic", "ollama", "mcp"}
		for _, backend := range validBackends {
			config := &Config{
				Models: ModelsConfig{
					Lead: "openai:gpt-4",
				},
				Backend: backend,
			}

			if backend == "mcp" {
				config.MCP = &MCPConfig{
					ServerURL: "http://localhost:8080",
				}
			}

			err := loader.validate(config)
			assert.NoError(t, err, "Backend %s should be valid", backend)
		}
	})

	t.Run("MCP backend without config fails", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
			},
			Backend: "mcp",
			MCP:     nil,
		}

		err := loader.validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MCP configuration required")
	})

	t.Run("MCP backend without server URL fails", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
			},
			Backend: "mcp",
			MCP: &MCPConfig{
				ServerURL: "",
			},
		}

		err := loader.validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MCP server URL must be specified")
	})

	t.Run("auto-fix invalid memory max entries", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
			},
			Memory: MemoryConfig{
				MaxEntries: 0,
			},
		}

		err := loader.validate(config)
		assert.NoError(t, err)
		assert.Equal(t, 10, config.Memory.MaxEntries)
	})

	t.Run("auto-fix invalid sandbox timeout", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
			},
			Sandbox: SandboxConfig{
				Timeout: 0,
			},
		}

		err := loader.validate(config)
		assert.NoError(t, err)
		assert.Equal(t, 5*time.Minute, config.Sandbox.Timeout)
	})
}

func TestLoaderExpandPaths(t *testing.T) {
	loader := NewLoader()

	t.Run("expand tilde paths", func(t *testing.T) {
		config := &Config{
			Memory: MemoryConfig{
				GlobalPath: "~/global",
				LocalPath:  "./local",
			},
		}

		loader.expandPaths(config)

		// GlobalPath should be expanded
		assert.NotEqual(t, "~/global", config.Memory.GlobalPath)
		assert.Contains(t, config.Memory.GlobalPath, "global")

		// LocalPath should remain unchanged
		assert.Equal(t, "./local", config.Memory.LocalPath)
	})

	t.Run("paths without tilde unchanged", func(t *testing.T) {
		config := &Config{
			Memory: MemoryConfig{
				GlobalPath: "/absolute/path",
				LocalPath:  "relative/path",
			},
		}

		originalGlobal := config.Memory.GlobalPath
		originalLocal := config.Memory.LocalPath

		loader.expandPaths(config)

		assert.Equal(t, originalGlobal, config.Memory.GlobalPath)
		assert.Equal(t, originalLocal, config.Memory.LocalPath)
	})

	t.Run("empty paths unchanged", func(t *testing.T) {
		config := &Config{
			Memory: MemoryConfig{
				GlobalPath: "",
				LocalPath:  "",
			},
		}

		loader.expandPaths(config)

		assert.Equal(t, "", config.Memory.GlobalPath)
		assert.Equal(t, "", config.Memory.LocalPath)
	})
}

func TestExpandPath(t *testing.T) {
	t.Run("expand tilde", func(t *testing.T) {
		result := expandPath("~/test/path")
		assert.NotEqual(t, "~/test/path", result)
		assert.Contains(t, result, "test/path")
		assert.NotContains(t, result, "~")
	})

	t.Run("path without tilde unchanged", func(t *testing.T) {
		paths := []string{
			"/absolute/path",
			"relative/path",
			"./current/path",
			"../parent/path",
		}

		for _, path := range paths {
			result := expandPath(path)
			assert.Equal(t, path, result)
		}
	})

	t.Run("empty path unchanged", func(t *testing.T) {
		result := expandPath("")
		assert.Equal(t, "", result)
	})

	t.Run("tilde only", func(t *testing.T) {
		result := expandPath("~")
		assert.NotEqual(t, "~", result)
		assert.NotEmpty(t, result)
	})
}

func TestLoaderSave(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader()

	t.Run("save config successfully", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "anthropic:claude-3",
			},
			Logging: LoggingConfig{
				Level:  "debug",
				Format: "json",
			},
		}

		savePath := filepath.Join(tmpDir, "saved.yml")
		err := loader.Save(config, savePath)
		require.NoError(t, err)

		// Verify file exists
		assert.FileExists(t, savePath)

		// Load and verify content
		loadedConfig, err := loader.loadFromFile(savePath)
		require.NoError(t, err)
		assert.Equal(t, "anthropic:claude-3", loadedConfig.Models.Lead)
		assert.Equal(t, "debug", loadedConfig.Logging.Level)
		assert.Equal(t, "json", loadedConfig.Logging.Format)
	})

	t.Run("save to nested directory", func(t *testing.T) {
		config := &defaultConfig
		nestedPath := filepath.Join(tmpDir, "deep", "nested", "config.yml")

		err := loader.Save(config, nestedPath)
		require.NoError(t, err)
		assert.FileExists(t, nestedPath)
	})

	t.Run("save with proper permissions", func(t *testing.T) {
		config := &defaultConfig
		savePath := filepath.Join(tmpDir, "permissions.yml")

		err := loader.Save(config, savePath)
		require.NoError(t, err)

		// Check file permissions
		info, err := os.Stat(savePath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})
}

func TestCreateDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()

	// Change to temporary directory
	err := os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	t.Run("create default config", func(t *testing.T) {
		err := CreateDefaultConfig()
		require.NoError(t, err)

		// Verify file was created
		configPath := ".sigil/config.yml"
		assert.FileExists(t, configPath)

		// Load and verify it's the default config
		loader := NewLoader()
		config, err := loader.loadFromFile(configPath)
		require.NoError(t, err)
		assert.Equal(t, defaultConfig.Models.Lead, config.Models.Lead)
		assert.Equal(t, defaultConfig.Logging.Level, config.Logging.Level)
	})
}
