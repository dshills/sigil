package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dshills/sigil/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Create temporary directory for tests
	tmpDir := t.TempDir()

	t.Run("load existing config file", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "config.yml")
		configContent := `
models:
  lead: "openai:gpt-4"
  reviewers:
    - "anthropic:claude-3"
logging:
  level: "debug"
  format: "json"
sandbox:
  enabled: true
  timeout: 300s
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "openai:gpt-4", config.Models.Lead)
		assert.Equal(t, []string{"anthropic:claude-3"}, config.Models.Reviewers)
		assert.Equal(t, "debug", config.Logging.Level)
		assert.Equal(t, "json", config.Logging.Format)
		assert.True(t, config.Sandbox.Enabled)
		assert.Equal(t, 300*time.Second, config.Sandbox.Timeout)
	})

	t.Run("load non-existent file returns defaults", func(t *testing.T) {
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.yml")
		config, err := Load(nonExistentPath)
		require.NoError(t, err)
		assert.NotNil(t, config)
		// Should return default config
		assert.Equal(t, "openai:gpt-4", config.Models.Lead)
		assert.Equal(t, "info", config.Logging.Level)
	})

	t.Run("load invalid YAML file", func(t *testing.T) {
		invalidPath := filepath.Join(tmpDir, "invalid.yml")
		invalidContent := `
models:
  lead: "openai:gpt-4"
  invalid_yaml: [
`
		err := os.WriteFile(invalidPath, []byte(invalidContent), 0644)
		require.NoError(t, err)

		config, err := Load(invalidPath)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "failed to parse config file")
	})

	t.Run("load config with invalid model format", func(t *testing.T) {
		invalidModelPath := filepath.Join(tmpDir, "invalid_model.yml")
		invalidModelContent := `
models:
  lead: "invalid-model-format"
`
		err := os.WriteFile(invalidModelPath, []byte(invalidModelContent), 0644)
		require.NoError(t, err)

		config, err := Load(invalidModelPath)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "invalid configuration")
	})
}

func TestParse(t *testing.T) {
	t.Run("parse valid YAML", func(t *testing.T) {
		yamlContent := `
models:
  lead: "openai:gpt-4"
logging:
  level: "warn"
`
		reader := strings.NewReader(yamlContent)
		config, err := Parse(reader)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "openai:gpt-4", config.Models.Lead)
		assert.Equal(t, "warn", config.Logging.Level)
	})

	t.Run("parse minimal YAML returns defaults", func(t *testing.T) {
		reader := strings.NewReader("{}")
		config, err := Parse(reader)
		require.NoError(t, err)
		assert.NotNil(t, config)
		// Should have default values
		assert.Equal(t, "openai:gpt-4", config.Models.Lead)
		assert.Equal(t, "info", config.Logging.Level)
	})

	t.Run("parse invalid YAML", func(t *testing.T) {
		invalidYaml := "invalid: yaml: content: ["
		reader := strings.NewReader(invalidYaml)
		config, err := Parse(reader)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "failed to decode YAML")
	})
}

func TestConfigSave(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("save valid config", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "anthropic:claude-3",
			},
			Logging: LoggingConfig{
				Level:  "debug",
				Format: "json",
			},
		}

		savePath := filepath.Join(tmpDir, "saved_config.yml")
		err := config.Save(savePath)
		require.NoError(t, err)

		// Verify file was created
		assert.FileExists(t, savePath)

		// Load it back and verify
		loadedConfig, err := Load(savePath)
		require.NoError(t, err)
		assert.Equal(t, "anthropic:claude-3", loadedConfig.Models.Lead)
		assert.Equal(t, "debug", loadedConfig.Logging.Level)
		assert.Equal(t, "json", loadedConfig.Logging.Format)
	})

	t.Run("save to nested directory", func(t *testing.T) {
		config := &defaultConfig
		nestedPath := filepath.Join(tmpDir, "nested", "deep", "config.yml")
		
		err := config.Save(nestedPath)
		require.NoError(t, err)
		assert.FileExists(t, nestedPath)
	})
}

func TestConfigValidate(t *testing.T) {
	t.Run("valid config passes validation", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
				Reviewers: []string{"anthropic:claude-3"},
			},
			Logging: LoggingConfig{
				Level: "info",
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("empty lead model fails validation", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "",
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "lead model is required")
	})

	t.Run("invalid lead model format fails validation", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "invalid-format",
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid lead model format")
	})

	t.Run("invalid reviewer model format fails validation", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
				Reviewers: []string{"invalid-format"},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid reviewer model format")
	})

	t.Run("invalid log level fails validation", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
			},
			Logging: LoggingConfig{
				Level: "invalid",
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid log level")
	})

	t.Run("MCP backend without config fails validation", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
			},
			Logging: LoggingConfig{
				Level: "info",
			},
			Backend: "mcp",
			MCP:     nil,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MCP configuration required")
	})

	t.Run("MCP backend with config passes validation", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead: "openai:gpt-4",
			},
			Logging: LoggingConfig{
				Level: "info",
			},
			Backend: "mcp",
			MCP: &MCPConfig{
				ServerURL: "http://localhost:8080",
				Model:     "test-model",
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})
}

func TestGlobalConfig(t *testing.T) {
	// Reset global state before tests
	originalConfig := globalConfig
	defer func() {
		globalConfig = originalConfig
	}()

	t.Run("get default config when none set", func(t *testing.T) {
		globalConfig = nil
		config := Get()
		assert.NotNil(t, config)
		assert.Equal(t, defaultConfig.Models.Lead, config.Models.Lead)
	})

	t.Run("get and set global config", func(t *testing.T) {
		testConfig := &Config{
			Models: ModelsConfig{
				Lead: "test:model",
			},
		}

		Set(testConfig)
		retrievedConfig := Get()
		assert.Equal(t, testConfig, retrievedConfig)
		assert.Equal(t, "test:model", retrievedConfig.Models.Lead)
	})
}

func TestApplyEnvOverrides(t *testing.T) {
	// Save original environment
	originalModel := os.Getenv("SIGIL_MODEL")
	originalLevel := os.Getenv("SIGIL_LOG_LEVEL")
	originalOpenAI := os.Getenv("OPENAI_API_KEY")
	originalAnthropic := os.Getenv("ANTHROPIC_API_KEY")

	// Clean up after test
	defer func() {
		os.Setenv("SIGIL_MODEL", originalModel)
		os.Setenv("SIGIL_LOG_LEVEL", originalLevel)
		os.Setenv("OPENAI_API_KEY", originalOpenAI)
		os.Setenv("ANTHROPIC_API_KEY", originalAnthropic)
	}()

	t.Run("override model from environment", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Lead:    "original:model",
				Configs: make(map[string]model.ModelConfig),
			},
		}

		os.Setenv("SIGIL_MODEL", "env:model")
		applyEnvOverrides(config)

		assert.Equal(t, "env:model", config.Models.Lead)
	})

	t.Run("override log level from environment", func(t *testing.T) {
		config := &Config{
			Logging: LoggingConfig{
				Level: "info",
			},
		}

		os.Setenv("SIGIL_LOG_LEVEL", "debug")
		applyEnvOverrides(config)

		assert.Equal(t, "debug", config.Logging.Level)
	})

	t.Run("set API keys from environment", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Configs: make(map[string]model.ModelConfig),
			},
		}

		os.Setenv("OPENAI_API_KEY", "test-openai-key")
		os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
		applyEnvOverrides(config)

		assert.Equal(t, "test-openai-key", config.Models.Configs["openai"].APIKey)
		assert.Equal(t, "test-anthropic-key", config.Models.Configs["anthropic"].APIKey)
	})

	t.Run("no environment variables set", func(t *testing.T) {
		originalConfig := &Config{
			Models: ModelsConfig{
				Lead: "original:model",
				Configs: make(map[string]model.ModelConfig),
			},
			Logging: LoggingConfig{
				Level: "info",
			},
		}

		os.Unsetenv("SIGIL_MODEL")
		os.Unsetenv("SIGIL_LOG_LEVEL")
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("ANTHROPIC_API_KEY")

		applyEnvOverrides(originalConfig)

		// Should remain unchanged
		assert.Equal(t, "original:model", originalConfig.Models.Lead)
		assert.Equal(t, "info", originalConfig.Logging.Level)
	})
}

func TestEnsureModelConfig(t *testing.T) {
	t.Run("create new config map if nil", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Configs: nil,
			},
		}

		ensureModelConfig(config, "openai")

		assert.NotNil(t, config.Models.Configs)
		assert.Contains(t, config.Models.Configs, "openai")
		assert.Equal(t, "openai", config.Models.Configs["openai"].Provider)
	})

	t.Run("add config for new provider", func(t *testing.T) {
		config := &Config{
			Models: ModelsConfig{
				Configs: make(map[string]model.ModelConfig),
			},
		}

		ensureModelConfig(config, "anthropic")

		assert.Contains(t, config.Models.Configs, "anthropic")
		assert.Equal(t, "anthropic", config.Models.Configs["anthropic"].Provider)
	})

	t.Run("preserve existing config", func(t *testing.T) {
		existingConfig := model.ModelConfig{
			Provider: "openai",
			APIKey:   "existing-key",
		}
		config := &Config{
			Models: ModelsConfig{
				Configs: map[string]model.ModelConfig{
					"openai": existingConfig,
				},
			},
		}

		ensureModelConfig(config, "openai")

		assert.Equal(t, existingConfig, config.Models.Configs["openai"])
		assert.Equal(t, "existing-key", config.Models.Configs["openai"].APIKey)
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("default config is valid", func(t *testing.T) {
		config := defaultConfig
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("default config has expected values", func(t *testing.T) {
		config := defaultConfig

		assert.Equal(t, "openai:gpt-4", config.Models.Lead)
		assert.True(t, config.Sandbox.Enabled)
		assert.Equal(t, 5*time.Minute, config.Sandbox.Timeout)
		assert.True(t, config.Sandbox.Cleanup)
		assert.True(t, config.Memory.Enabled)
		assert.Equal(t, 10, config.Memory.MaxEntries)
		assert.Equal(t, "info", config.Logging.Level)
		assert.Equal(t, "text", config.Logging.Format)
		assert.False(t, config.Git.AutoCommit)
		assert.True(t, config.Git.Checkpoints)
		assert.Equal(t, "sigil: %s", config.Git.CommitTemplate)
	})
}

func TestConfigTypes(t *testing.T) {
	t.Run("rule struct", func(t *testing.T) {
		rule := Rule{
			Name:     "test-rule",
			Command:  "go test",
			MustPass: true,
			Timeout:  30 * time.Second,
		}

		assert.Equal(t, "test-rule", rule.Name)
		assert.Equal(t, "go test", rule.Command)
		assert.True(t, rule.MustPass)
		assert.Equal(t, 30*time.Second, rule.Timeout)
	})

	t.Run("MCP config struct", func(t *testing.T) {
		mcpConfig := MCPConfig{
			ServerURL: "http://localhost:8080",
			Model:     "test-model",
			APIKey:    "test-key",
			Settings:  map[string]interface{}{"setting1": "value1"},
		}

		assert.Equal(t, "http://localhost:8080", mcpConfig.ServerURL)
		assert.Equal(t, "test-model", mcpConfig.Model)
		assert.Equal(t, "test-key", mcpConfig.APIKey)
		assert.Equal(t, "value1", mcpConfig.Settings["setting1"])
	})
}