package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dshills/sigil/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	provider := NewProvider()
	
	assert.NotNil(t, provider)
	assert.NotNil(t, provider.client)
	assert.Equal(t, defaultTimeout, provider.client.Timeout)
}

func TestProvider_Name(t *testing.T) {
	provider := NewProvider()
	assert.Equal(t, "anthropic", provider.Name())
}

func TestProvider_ListModels(t *testing.T) {
	provider := NewProvider()
	ctx := context.Background()
	
	models, err := provider.ListModels(ctx)
	
	assert.NoError(t, err)
	assert.NotEmpty(t, models)
	assert.Contains(t, models, "claude-3-opus-20240229")
	assert.Contains(t, models, "claude-3-sonnet-20240229")
	assert.Contains(t, models, "claude-3-haiku-20240307")
}

func TestProvider_CreateModel(t *testing.T) {
	provider := NewProvider()
	
	t.Run("valid config", func(t *testing.T) {
		config := model.ModelConfig{
			APIKey: "test-api-key",
			Model:  "claude-3-5-sonnet-20241022",
		}
		
		model, err := provider.CreateModel(config)
		
		assert.NoError(t, err)
		assert.NotNil(t, model)
		
		anthropicModel, ok := model.(*Model)
		assert.True(t, ok)
		assert.Equal(t, "test-api-key", anthropicModel.apiKey)
		assert.Equal(t, "claude-3-5-sonnet-20241022", anthropicModel.modelName)
		assert.Equal(t, defaultBaseURL, anthropicModel.baseURL)
	})
	
	t.Run("missing API key", func(t *testing.T) {
		config := model.ModelConfig{
			Model: "claude-3-5-sonnet-20241022",
		}
		
		model, err := provider.CreateModel(config)
		
		assert.Error(t, err)
		assert.Nil(t, model)
		assert.Contains(t, err.Error(), "API key is required")
	})
	
	t.Run("custom endpoint", func(t *testing.T) {
		config := model.ModelConfig{
			APIKey:   "test-api-key",
			Model:    "claude-3-5-sonnet-20241022",
			Endpoint: "https://custom.anthropic.com/v1",
		}
		
		model, err := provider.CreateModel(config)
		
		assert.NoError(t, err)
		assert.NotNil(t, model)
		
		anthropicModel, ok := model.(*Model)
		assert.True(t, ok)
		assert.Equal(t, "https://custom.anthropic.com/v1", anthropicModel.baseURL)
	})
	
	t.Run("custom timeout", func(t *testing.T) {
		customTimeout := 60 * time.Second
		config := model.ModelConfig{
			APIKey: "test-api-key",
			Model:  "claude-3-5-sonnet-20241022",
			Options: map[string]interface{}{
				"timeout": customTimeout,
			},
		}
		
		model, err := provider.CreateModel(config)
		
		assert.NoError(t, err)
		assert.NotNil(t, model)
		
		anthropicModel, ok := model.(*Model)
		assert.True(t, ok)
		assert.Equal(t, customTimeout, anthropicModel.client.Timeout)
	})
}

func TestModel_GetCapabilities(t *testing.T) {
	provider := NewProvider()
	config := model.ModelConfig{
		APIKey: "test-api-key",
		Model:  "claude-3-5-sonnet-20241022",
	}
	
	model, err := provider.CreateModel(config)
	require.NoError(t, err)
	
	capabilities := model.GetCapabilities()
	
	assert.Equal(t, 200000, capabilities.MaxTokens) // Claude-3 model
	assert.True(t, capabilities.SupportsImages) // Claude-3 supports vision
	assert.True(t, capabilities.SupportsTools)
	assert.False(t, capabilities.SupportsStreaming)
}

func TestModel_Name(t *testing.T) {
	provider := NewProvider()
	config := model.ModelConfig{
		APIKey: "test-api-key",
		Model:  "claude-3-5-sonnet-20241022",
	}
	
	model, err := provider.CreateModel(config)
	require.NoError(t, err)
	
	assert.Equal(t, "anthropic:claude-3-5-sonnet-20241022", model.Name())
}

func TestModel_RunPrompt(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/messages", r.URL.Path)
			assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))
			assert.Equal(t, apiVersion, r.Header.Get("anthropic-version"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			
			// Parse request body
			var req MessageRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)
			assert.Equal(t, "claude-3-5-sonnet-20241022", req.Model)
			assert.Equal(t, 1000, req.MaxTokens)
			assert.Equal(t, float32(0.7), req.Temperature)
			assert.Len(t, req.Messages, 1)
			assert.Equal(t, "user", req.Messages[0].Role)
			
			// Send mock response
			response := MessageResponse{
				ID:    "msg_123",
				Type:  "message",
				Role:  "assistant",
				Model: "claude-3-5-sonnet-20241022",
				Content: []ContentBlock{
					{
						Type: "text",
						Text: "This is a test response from Claude",
					},
				},
				Usage: Usage{
					InputTokens:  25,
					OutputTokens: 15,
				},
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		
		// Create model with mock server
		provider := NewProvider()
		config := model.ModelConfig{
			APIKey:   "test-api-key",
			Model:    "claude-3-5-sonnet-20241022",
			Endpoint: server.URL,
		}
		
		modelInstance, err := provider.CreateModel(config)
		require.NoError(t, err)
		
		// Test prompt
		input := model.PromptInput{
			SystemPrompt: "You are a helpful assistant.",
			UserPrompt:   "Hello, how are you?",
			MaxTokens:    1000,
			Temperature:  0.7,
		}
		
		ctx := context.Background()
		output, err := modelInstance.RunPrompt(ctx, input)
		
		assert.NoError(t, err)
		assert.Equal(t, "This is a test response from Claude", output.Response)
		assert.Equal(t, 40, output.TokensUsed) // InputTokens + OutputTokens
		assert.Equal(t, "claude-3-5-sonnet-20241022", output.Model)
	})
	
	t.Run("API error", func(t *testing.T) {
		// Create mock server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"type": "error", "error": {"type": "authentication_error", "message": "Invalid API key"}}`))
		}))
		defer server.Close()
		
		// Create model with mock server
		provider := NewProvider()
		config := model.ModelConfig{
			APIKey:   "invalid-key",
			Model:    "claude-3-5-sonnet-20241022",
			Endpoint: server.URL,
		}
		
		modelInstance, err := provider.CreateModel(config)
		require.NoError(t, err)
		
		// Test prompt
		input := model.PromptInput{
			SystemPrompt: "You are a helpful assistant.",
			UserPrompt:   "Hello",
		}
		
		ctx := context.Background()
		output, err := modelInstance.RunPrompt(ctx, input)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API error 401:")
		assert.Empty(t, output.Response)
	})
	
	t.Run("with files", func(t *testing.T) {
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse request body to verify files are included
			var req MessageRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)
			
			// Should have user message with file content
			assert.Len(t, req.Messages, 1)
			assert.Equal(t, "user", req.Messages[0].Role)
			
			// Check that file content is included in content blocks
			found := false
			for _, block := range req.Messages[0].Content {
				if strings.Contains(block.Text, "package main") {
					found = true
					break
				}
			}
			assert.True(t, found, "File content should be included in message content blocks")
			
			// Send response
			response := MessageResponse{
				ID:   "msg_123",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{
						Type: "text",
						Text: "I can see the Go code you provided.",
					},
				},
				Usage: Usage{
					InputTokens:  30,
					OutputTokens: 20,
				},
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		
		// Create model with mock server
		provider := NewProvider()
		config := model.ModelConfig{
			APIKey:   "test-api-key",
			Model:    "claude-3-5-sonnet-20241022",
			Endpoint: server.URL,
		}
		
		modelInstance, err := provider.CreateModel(config)
		require.NoError(t, err)
		
		// Test prompt with files
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
		
		assert.NoError(t, err)
		assert.Equal(t, "I can see the Go code you provided.", output.Response)
		assert.Equal(t, 50, output.TokensUsed)
	})
}

func TestModel_buildRequest(t *testing.T) {
	provider := NewProvider()
	config := model.ModelConfig{
		APIKey: "test-api-key",
		Model:  "claude-3-5-sonnet-20241022",
	}
	
	modelInstance, err := provider.CreateModel(config)
	require.NoError(t, err)
	
	anthropicModel := modelInstance.(*Model)
	
	t.Run("basic request", func(t *testing.T) {
		input := model.PromptInput{
			SystemPrompt: "You are helpful.",
			UserPrompt:   "Hello",
			MaxTokens:    1000,
			Temperature:  0.7,
		}
		
		req := anthropicModel.buildRequest(input)
		
		assert.Equal(t, "claude-3-5-sonnet-20241022", req.Model)
		assert.Equal(t, 1000, req.MaxTokens)
		assert.Equal(t, float32(0.7), req.Temperature)
		assert.Equal(t, "You are helpful.", req.System)
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Len(t, req.Messages[0].Content, 1)
		assert.Equal(t, "Hello", req.Messages[0].Content[0].Text)
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
		
		req := anthropicModel.buildRequest(input)
		
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)
		// Check content blocks contain file information
		found := false
		for _, block := range req.Messages[0].Content {
			if strings.Contains(block.Text, "test.go") && strings.Contains(block.Text, "package test") {
				found = true
				break
			}
		}
		assert.True(t, found, "File content should be included in message content blocks")
	})
	
	t.Run("default values", func(t *testing.T) {
		input := model.PromptInput{
			UserPrompt: "Hello",
		}
		
		req := anthropicModel.buildRequest(input)
		
		assert.Equal(t, 1000, req.MaxTokens) // default
		assert.Equal(t, float32(0.7), req.Temperature) // default
	})
}

func TestMessage_Structure(t *testing.T) {
	msg := Message{
		Role: "user",
		Content: []ContentBlock{
			{
				Type: "text",
				Text: "Hello, world!",
			},
		},
	}
	
	assert.Equal(t, "user", msg.Role)
	assert.Len(t, msg.Content, 1)
	assert.Equal(t, "text", msg.Content[0].Type)
	assert.Equal(t, "Hello, world!", msg.Content[0].Text)
}

func TestMessageRequest_Structure(t *testing.T) {
	req := MessageRequest{
		Model:       "claude-3-5-sonnet-20241022",
		MaxTokens:   1000,
		Temperature: 0.7,
		System:      "You are helpful",
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{
						Type: "text",
						Text: "test",
					},
				},
			},
		},
	}
	
	assert.Equal(t, "claude-3-5-sonnet-20241022", req.Model)
	assert.Equal(t, 1000, req.MaxTokens)
	assert.Equal(t, float32(0.7), req.Temperature)
	assert.Equal(t, "You are helpful", req.System)
	assert.Len(t, req.Messages, 1)
	assert.Equal(t, "user", req.Messages[0].Role)
	assert.Len(t, req.Messages, 1)
}

func TestMessageResponse_Structure(t *testing.T) {
	response := MessageResponse{
		ID:    "msg_123",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-5-sonnet-20241022",
		Content: []ContentBlock{
			{
				Type: "text",
				Text: "Test response",
			},
		},
		Usage: Usage{
			InputTokens:  25,
			OutputTokens: 15,
		},
	}
	
	assert.Equal(t, "msg_123", response.ID)
	assert.Equal(t, "message", response.Type)
	assert.Equal(t, "assistant", response.Role)
	assert.Equal(t, "claude-3-5-sonnet-20241022", response.Model)
	assert.Len(t, response.Content, 1)
	assert.Equal(t, "Test response", response.Content[0].Text)
	assert.Equal(t, 25, response.Usage.InputTokens)
	assert.Equal(t, 15, response.Usage.OutputTokens)
}

func TestContentBlock_Structure(t *testing.T) {
	block := ContentBlock{
		Type: "text",
		Text: "Hello world",
	}
	
	assert.Equal(t, "text", block.Type)
	assert.Equal(t, "Hello world", block.Text)
}