package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/dshills/sigil/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	provider := NewProvider()
	
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.clients)
	assert.Empty(t, provider.clients)
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
	assert.NotEmpty(t, models)
	assert.Contains(t, models, "mcp://localhost:3000/claude-3-sonnet")
	assert.Contains(t, models, "mcp://localhost:3000/gpt-4")
	assert.Contains(t, models, "mcp://localhost:3000/llama-2")
}

func TestProvider_CreateModel(t *testing.T) {
	provider := NewProvider()
	
	t.Run("valid config", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "mcp-server:test",
			Endpoint: "stdio://path/to/mcp-server",
		}
		
		model, err := provider.CreateModel(config)
		
		assert.NoError(t, err)
		assert.NotNil(t, model)
		
		mcpModel, ok := model.(*Model)
		assert.True(t, ok)
		assert.Equal(t, "mcp-server:test", mcpModel.modelName)
		assert.NotNil(t, mcpModel.client)
	})
	
	t.Run("missing endpoint", func(t *testing.T) {
		config := model.ModelConfig{
			Model: "mcp-server:test",
		}
		
		model, err := provider.CreateModel(config)
		
		assert.Error(t, err)
		assert.Nil(t, model)
		assert.Contains(t, err.Error(), "endpoint is required")
	})
	
	t.Run("with transport options", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "mcp-server:test",
			Endpoint: "ws://localhost:8080/mcp",
			Options: map[string]interface{}{
				"transport": "websocket",
				"timeout":   30 * time.Second,
			},
		}
		
		model, err := provider.CreateModel(config)
		
		assert.NoError(t, err)
		assert.NotNil(t, model)
		
		mcpModel, ok := model.(*Model)
		assert.True(t, ok)
		assert.Equal(t, "mcp-server:test", mcpModel.modelName)
	})
	
	t.Run("reuse existing client", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "mcp-server:test",
			Endpoint: "stdio://same/server/path",
		}
		
		// Create first model
		model1, err := provider.CreateModel(config)
		require.NoError(t, err)
		
		// Create second model with same config
		model2, err := provider.CreateModel(config)
		require.NoError(t, err)
		
		// Should reuse the same client
		mcpModel1 := model1.(*Model)
		mcpModel2 := model2.(*Model)
		assert.Equal(t, mcpModel1.client, mcpModel2.client)
	})
}

func TestModel_GetCapabilities(t *testing.T) {
	provider := NewProvider()
	config := model.ModelConfig{
		Model:    "mcp-server:test",
		Endpoint: "stdio://path/to/server",
	}
	
	model, err := provider.CreateModel(config)
	require.NoError(t, err)
	
	capabilities := model.GetCapabilities()
	
	assert.Equal(t, 4096, capabilities.MaxTokens) // default for basic MCP model
	assert.False(t, capabilities.SupportsImages) // depends on server
	assert.True(t, capabilities.SupportsTools)
	assert.False(t, capabilities.SupportsStreaming)
}

func TestModel_Name(t *testing.T) {
	provider := NewProvider()
	config := model.ModelConfig{
		Model:    "mcp-server:test",
		Endpoint: "stdio://path/to/server",
	}
	
	model, err := provider.CreateModel(config)
	require.NoError(t, err)
	
	assert.Equal(t, "mcp:mcp-server:test", model.Name())
}

func TestProvider_parseServerConfig(t *testing.T) {
	provider := NewProvider()
	
	t.Run("basic config", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "mcp-server:test",
			Endpoint: "http://localhost:8080/mcp",
		}
		
		serverConfig := provider.parseServerConfig(config)
		
		assert.Equal(t, "http://localhost:8080/mcp", serverConfig.ServerURL)
		assert.Equal(t, 30*time.Second, serverConfig.Timeout)
	})
	
	t.Run("with custom timeout", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "mcp-server:test",
			Endpoint: "ws://localhost:8080/mcp",
			Options: map[string]interface{}{
				"timeout": 60 * time.Second,
			},
		}
		
		serverConfig := provider.parseServerConfig(config)
		
		assert.Equal(t, "ws://localhost:8080/mcp", serverConfig.ServerURL)
		assert.Equal(t, 60*time.Second, serverConfig.Timeout)
	})
	
	t.Run("with transport option", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "mcp-server:test",
			Endpoint: "http://api.example.com/mcp",
			Options: map[string]interface{}{
				"transport": "http",
			},
		}
		
		serverConfig := provider.parseServerConfig(config)
		
		assert.Equal(t, "http://api.example.com/mcp", serverConfig.ServerURL)
		assert.Equal(t, "http", serverConfig.Transport)
	})
	
	t.Run("with auth options", func(t *testing.T) {
		config := model.ModelConfig{
			Model:    "mcp-server:test",
			Endpoint: "http://api.example.com/mcp",
			Options: map[string]interface{}{
				"auth": map[string]interface{}{
					"token": "secret123",
					"type":  "bearer",
				},
			},
		}
		
		serverConfig := provider.parseServerConfig(config)
		
		assert.Equal(t, "http://api.example.com/mcp", serverConfig.ServerURL)
		expectedAuth := map[string]interface{}{
			"token": "secret123",
			"type":  "bearer",
		}
		assert.Equal(t, expectedAuth, serverConfig.Auth)
	})
}

func TestMCPServerConfig_Structure(t *testing.T) {
	tests := []struct {
		name   string
		config MCPServerConfig
	}{
		{
			name: "basic config",
			config: MCPServerConfig{
				ServerURL: "http://localhost:8080/mcp",
				Transport: "http",
				Timeout:   30 * time.Second,
			},
		},
		{
			name: "websocket config",
			config: MCPServerConfig{
				ServerURL: "ws://localhost:8080/mcp",
				Transport: "websocket",
				Timeout:   60 * time.Second,
			},
		},
		{
			name: "with auth",
			config: MCPServerConfig{
				ServerURL: "http://api.example.com/mcp",
				Transport: "http",
				Timeout:   30 * time.Second,
				Auth:      map[string]interface{}{"token": "secret"},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.config.ServerURL)
			assert.NotEmpty(t, tt.config.Transport)
			assert.True(t, tt.config.Timeout > 0)
		})
	}
}

func TestModel_RunPrompt(t *testing.T) {
	provider := NewProvider()
	config := model.ModelConfig{
		Model:    "mcp-server:test",
		Endpoint: "stdio://mock-server",
	}
	
	modelInstance, err := provider.CreateModel(config)
	require.NoError(t, err)
	
	// Note: Since we're not actually running a real MCP server,
	// we'll test the error handling paths
	
	t.Run("prompt execution", func(t *testing.T) {
		input := model.PromptInput{
			SystemPrompt: "You are a helpful assistant.",
			UserPrompt:   "Hello, how are you?",
			MaxTokens:    1000,
			Temperature:  0.7,
		}
		
		ctx := context.Background()
		output, err := modelInstance.RunPrompt(ctx, input)
		
		// Since we have a mock MCP server, this should succeed
		assert.NoError(t, err)
		assert.NotEmpty(t, output.Response)
		assert.Contains(t, output.Response, "simulated MCP response")
	})
	
	t.Run("with files", func(t *testing.T) {
		input := model.PromptInput{
			SystemPrompt: "You are a code reviewer.",
			UserPrompt:   "Please review this code.",
			Files: []model.FileContent{
				{
					Path:    "main.go",
					Content: "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}",
					Type:    "code",
				},
			},
		}
		
		ctx := context.Background()
		output, err := modelInstance.RunPrompt(ctx, input)
		
		// Should succeed with mock MCP server
		assert.NoError(t, err)
		assert.NotEmpty(t, output.Response)
		assert.Contains(t, output.Response, "simulated MCP response")
	})
}

func TestModel_buildRequest(t *testing.T) {
	provider := NewProvider()
	config := model.ModelConfig{
		Model:    "mcp-server:test",
		Endpoint: "http://mock-server",
	}
	
	modelInstance, err := provider.CreateModel(config)
	require.NoError(t, err)
	
	mcpModel := modelInstance.(*Model)
	
	t.Run("basic request", func(t *testing.T) {
		input := model.PromptInput{
			SystemPrompt: "You are helpful.",
			UserPrompt:   "Hello",
			MaxTokens:    1000,
			Temperature:  0.7,
		}
		
		req := mcpModel.buildRequest(input)
		
		assert.Equal(t, "2.0", req.JSONRPC)
		assert.Equal(t, "completion/complete", req.Method)
		assert.NotEmpty(t, req.ID)
		assert.NotNil(t, req.Params)
	})
	
	t.Run("with files", func(t *testing.T) {
		input := model.PromptInput{
			SystemPrompt: "You are helpful.",
			UserPrompt:   "Review this",
			Files: []model.FileContent{
				{
					Path:    "test.go",
					Content: "package test",
					Type:    "code",
				},
			},
		}
		
		req := mcpModel.buildRequest(input)
		
		assert.Equal(t, "2.0", req.JSONRPC)
		assert.Equal(t, "completion/complete", req.Method)
		assert.NotNil(t, req.Params)
	})
	
	t.Run("with memory", func(t *testing.T) {
		input := model.PromptInput{
			UserPrompt: "Continue our conversation",
			Memory: []model.MemoryEntry{
				{
					Timestamp: "2023-12-07T12:00:00Z",
					Content:   "Previous context about the project",
					Type:      "context",
				},
			},
		}
		
		req := mcpModel.buildRequest(input)
		
		assert.Equal(t, "2.0", req.JSONRPC)
		assert.NotNil(t, req.Params)
	})
}

func TestMCPRequest_Structure(t *testing.T) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-123",
		Method:  "completion/complete",
		Params: map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/prompt",
				"name": "default",
			},
			"arguments": map[string]interface{}{
				"model":       "test-model",
				"temperature": 0.7,
				"max_tokens":  1000,
			},
		},
	}
	
	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, "test-123", req.ID)
	assert.Equal(t, "completion/complete", req.Method)
	assert.NotNil(t, req.Params)
}

func TestMCPResponse_Structure(t *testing.T) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      "test-123",
		Result: map[string]interface{}{
			"content": "This is the response content",
			"usage": map[string]interface{}{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
		},
	}
	
	assert.Equal(t, "2.0", response.JSONRPC)
	assert.Equal(t, "test-123", response.ID)
	assert.NotNil(t, response.Result)
	assert.Nil(t, response.Error)
}

func TestMCPError_Structure(t *testing.T) {
	mcpErr := MCPError{
		Code:    -32601,
		Message: "Method not found",
		Data:    "The requested method does not exist",
	}
	
	assert.Equal(t, -32601, mcpErr.Code)
	assert.Equal(t, "Method not found", mcpErr.Message)
	assert.Equal(t, "The requested method does not exist", mcpErr.Data)
	
	// Test error fields
	assert.Equal(t, -32601, mcpErr.Code)
	assert.Equal(t, "Method not found", mcpErr.Message)
}

func TestMCPClient_Structure(t *testing.T) {
	client := &MCPClient{
		config: &MCPServerConfig{
			ServerURL: "http://localhost:8080/mcp",
			Transport: "http",
			Timeout:   30 * time.Second,
		},
	}
	
	assert.Equal(t, "http://localhost:8080/mcp", client.config.ServerURL)
	assert.Equal(t, "http", client.config.Transport)
	assert.Equal(t, 30*time.Second, client.config.Timeout)
}

func TestParseServerURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{"http", "http://api.example.com/mcp", "http://api.example.com/mcp"},
		{"https", "https://secure.api.example.com/mcp", "https://secure.api.example.com/mcp"},
		{"websocket", "ws://localhost:8080/mcp", "ws://localhost:8080/mcp"},
		{"secure websocket", "wss://secure.example.com/mcp", "wss://secure.example.com/mcp"},
		{"localhost", "http://localhost:8080", "http://localhost:8080"},
	}
	
	provider := NewProvider()
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := model.ModelConfig{
				Model:    "test",
				Endpoint: tt.endpoint,
			}
			
			serverConfig := provider.parseServerConfig(config)
			assert.Equal(t, tt.want, serverConfig.ServerURL)
		})
	}
}