package mcp

import (
	"context"
	"testing"

	"github.com/dshills/sigil/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	provider := NewProvider()

	assert.NotNil(t, provider)
	assert.NotNil(t, provider.processManager)
	assert.NotNil(t, provider.configLoader)
	assert.NotNil(t, provider.serverConfigs)
}

func TestProvider_Name(t *testing.T) {
	provider := NewProvider()
	assert.Equal(t, "mcp", provider.Name())
}

func TestProvider_ListModels(t *testing.T) {
	provider := NewProvider()
	ctx := context.Background()

	models, err := provider.ListModels(ctx)

	assert.NoError(t, err)
	// Should return placeholder models when no servers are running
	assert.NotEmpty(t, models)
}

func TestProvider_CreateModel(t *testing.T) {
	provider := NewProvider()

	t.Run("missing endpoint", func(t *testing.T) {
		config := model.ModelConfig{
			Model: "test-model",
		}

		model, err := provider.CreateModel(config)

		assert.Error(t, err)
		assert.Nil(t, model)
		assert.Contains(t, err.Error(), "endpoint is required")
	})

	t.Run("parse server name from endpoint", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "test-model",
			Endpoint: "mcp://test-server/model",
		}

		// This will fail because we don't have a real server configured
		// but it tests the parsing logic
		_, err := provider.CreateModel(config)
		assert.Error(t, err) // Expected to fail without real server
	})
}

func TestProvider_parseServerConfig(t *testing.T) {
	provider := NewProvider()

	t.Run("basic config", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "test-model",
			Endpoint: "mcp://test-server",
		}

		serverConfig := provider.parseServerConfig("test-server", config)

		assert.Equal(t, "test-server", serverConfig.Name)
		assert.Equal(t, "stdio", serverConfig.Transport)
		assert.True(t, serverConfig.AutoRestart)
		assert.Equal(t, 3, serverConfig.MaxRestarts)
	})

	t.Run("with command options", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "test-model",
			Endpoint: "mcp://custom-server",
			Options: map[string]interface{}{
				"command": "python",
				"args":    []string{"-m", "my_server"},
			},
		}

		serverConfig := provider.parseServerConfig("custom-server", config)

		assert.Equal(t, "custom-server", serverConfig.Name)
		assert.Equal(t, "python", serverConfig.Command)
		assert.Equal(t, []string{"-m", "my_server"}, serverConfig.Args)
	})

	t.Run("with environment", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "test-model",
			Endpoint: "mcp://env-server",
			Options: map[string]interface{}{
				"env": map[string]string{
					"API_KEY": "test-key",
					"DEBUG":   "true",
				},
			},
		}

		serverConfig := provider.parseServerConfig("env-server", config)

		assert.Equal(t, "env-server", serverConfig.Name)
		assert.Equal(t, "test-key", serverConfig.Env["API_KEY"])
		assert.Equal(t, "true", serverConfig.Env["DEBUG"])
	})

	t.Run("github-mcp default", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "test-model",
			Endpoint: "mcp://github-mcp",
		}

		serverConfig := provider.parseServerConfig("github-mcp", config)

		assert.Equal(t, "github-mcp", serverConfig.Name)
		assert.Equal(t, "npx", serverConfig.Command)
		assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-github"}, serverConfig.Args)
	})
}

func TestModel_GetCapabilities(t *testing.T) {
	// Create a mock server for testing
	server := &ManagedServer{
		Name: "test-server",
		Protocol: &ProtocolHandler{
			initialized: true,
			serverCaps: &ServerCapabilities{
				Streaming: false,
				Tools:     true,
				Resources: true,
			},
		},
	}

	model := &Model{
		modelName: "test-model",
		server:    server,
	}

	capabilities := model.GetCapabilities()

	assert.Equal(t, 8192, capabilities.MaxTokens)
	assert.False(t, capabilities.SupportsImages)
	assert.True(t, capabilities.SupportsTools)
	assert.False(t, capabilities.SupportsStreaming)
}

func TestModel_Name(t *testing.T) {
	server := &ManagedServer{
		Name: "test-server",
	}

	model := &Model{
		modelName: "test-model",
		server:    server,
	}

	assert.Equal(t, "mcp:test-server/test-model", model.Name())
}

func TestModel_buildMessages(t *testing.T) {
	mcpModel := &Model{
		modelName: "test-model",
	}

	t.Run("basic messages", func(t *testing.T) {
		input := model.PromptInput{
			SystemPrompt: "You are helpful.",
			UserPrompt:   "Hello",
		}

		messages := mcpModel.buildMessages(input)

		assert.Len(t, messages, 2)
		assert.Equal(t, "system", messages[0].Role)
		assert.Equal(t, "You are helpful.", messages[0].Content)
		assert.Equal(t, "user", messages[1].Role)
		assert.Equal(t, "Hello", messages[1].Content)
	})

	t.Run("with memory", func(t *testing.T) {
		input := model.PromptInput{
			SystemPrompt: "You are helpful.",
			Memory: []model.MemoryEntry{
				{
					Content: "Previous context",
					Type:    "context",
				},
			},
			UserPrompt: "Continue",
		}

		messages := mcpModel.buildMessages(input)

		assert.Len(t, messages, 3)
		assert.Equal(t, "system", messages[0].Role)
		assert.Equal(t, "assistant", messages[1].Role)
		assert.Equal(t, "Previous context", messages[1].Content)
		assert.Equal(t, "user", messages[2].Role)
	})

	t.Run("with files", func(t *testing.T) {
		input := model.PromptInput{
			UserPrompt: "Review this",
			Files: []model.FileContent{
				{
					Path:    "test.go",
					Content: "package test",
					Type:    "code",
				},
			},
		}

		messages := mcpModel.buildMessages(input)

		require.Len(t, messages, 1)
		assert.Equal(t, "user", messages[0].Role)
		assert.Contains(t, messages[0].Content, "Review this")
		assert.Contains(t, messages[0].Content, "test.go")
		assert.Contains(t, messages[0].Content, "package test")
	})
}

func TestConfigLoader(t *testing.T) {
	t.Run("create loader", func(t *testing.T) {
		loader := NewConfigLoader("/global/path", "/project/path")

		assert.NotNil(t, loader)
		assert.Equal(t, "/global/path", loader.globalPath)
		assert.Equal(t, "/project/path", loader.projectPath)
	})

	t.Run("default paths", func(t *testing.T) {
		globalPath, projectPath := GetDefaultPaths()

		assert.Contains(t, globalPath, ".config/sigil/mcp-servers.yml")
		assert.Equal(t, ".sigil/mcp-servers.yml", projectPath)
	})
}
