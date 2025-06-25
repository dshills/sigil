package openai

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
	assert.Equal(t, "openai", provider.Name())
}

func TestProvider_ListModels(t *testing.T) {
	provider := NewProvider()
	ctx := context.Background()

	models, err := provider.ListModels(ctx)

	assert.NoError(t, err)
	assert.NotEmpty(t, models)
	assert.Contains(t, models, "gpt-4")
	assert.Contains(t, models, "gpt-3.5-turbo")
	assert.Contains(t, models, "gpt-4-turbo")
}

func TestProvider_CreateModel(t *testing.T) {
	provider := NewProvider()

	t.Run("valid config", func(t *testing.T) {
		config := model.ModelConfig{
			APIKey: "test-api-key",
			Model:  "gpt-4",
		}

		model, err := provider.CreateModel(config)

		assert.NoError(t, err)
		assert.NotNil(t, model)

		openaiModel, ok := model.(*Model)
		assert.True(t, ok)
		assert.Equal(t, "test-api-key", openaiModel.apiKey)
		assert.Equal(t, "gpt-4", openaiModel.modelName)
		assert.Equal(t, defaultBaseURL, openaiModel.baseURL)
	})

	t.Run("missing API key", func(t *testing.T) {
		config := model.ModelConfig{
			Model: "gpt-4",
		}

		model, err := provider.CreateModel(config)

		assert.Error(t, err)
		assert.Nil(t, model)
		assert.Contains(t, err.Error(), "API key is required")
	})

	t.Run("custom endpoint", func(t *testing.T) {
		config := model.ModelConfig{
			APIKey:   "test-api-key",
			Model:    "gpt-4",
			Endpoint: "https://custom.openai.com/v1",
		}

		model, err := provider.CreateModel(config)

		assert.NoError(t, err)
		assert.NotNil(t, model)

		openaiModel, ok := model.(*Model)
		assert.True(t, ok)
		assert.Equal(t, "https://custom.openai.com/v1", openaiModel.baseURL)
	})

	t.Run("custom timeout", func(t *testing.T) {
		customTimeout := 60 * time.Second
		config := model.ModelConfig{
			APIKey: "test-api-key",
			Model:  "gpt-4",
			Options: map[string]interface{}{
				"timeout": customTimeout,
			},
		}

		model, err := provider.CreateModel(config)

		assert.NoError(t, err)
		assert.NotNil(t, model)

		openaiModel, ok := model.(*Model)
		assert.True(t, ok)
		assert.Equal(t, customTimeout, openaiModel.client.Timeout)
	})
}

func TestModel_GetCapabilities(t *testing.T) {
	provider := NewProvider()
	config := model.ModelConfig{
		APIKey: "test-api-key",
		Model:  "gpt-4",
	}

	model, err := provider.CreateModel(config)
	require.NoError(t, err)

	capabilities := model.GetCapabilities()

	assert.Equal(t, 8192, capabilities.MaxTokens) // gpt-4 model
	assert.False(t, capabilities.SupportsImages)
	assert.True(t, capabilities.SupportsTools)
	assert.False(t, capabilities.SupportsStreaming)
}

func TestModel_Name(t *testing.T) {
	provider := NewProvider()
	config := model.ModelConfig{
		APIKey: "test-api-key",
		Model:  "gpt-4",
	}

	model, err := provider.CreateModel(config)
	require.NoError(t, err)

	assert.Equal(t, "openai:gpt-4", model.Name())
}

func TestModel_RunPrompt(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/chat/completions", r.URL.Path)
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Parse request body
			var req ChatCompletionRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)
			assert.Equal(t, "gpt-4", req.Model)
			assert.Equal(t, 1000, req.MaxTokens)
			assert.Equal(t, float32(0.7), req.Temperature)
			assert.Len(t, req.Messages, 2)

			// Send mock response
			response := ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677652288,
				Model:   "gpt-4",
				Choices: []Choice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: "This is a test response from GPT-4",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{
					PromptTokens:     25,
					CompletionTokens: 15,
					TotalTokens:      40,
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
			Model:    "gpt-4",
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
		assert.Equal(t, "This is a test response from GPT-4", output.Response)
		assert.Equal(t, 40, output.TokensUsed)
		assert.Equal(t, "gpt-4", output.Model)
	})

	t.Run("API error", func(t *testing.T) {
		// Create mock server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
		}))
		defer server.Close()

		// Create model with mock server
		provider := NewProvider()
		config := model.ModelConfig{
			APIKey:   "invalid-key",
			Model:    "gpt-4",
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

	t.Run("network error", func(t *testing.T) {
		// Create model with invalid endpoint
		provider := NewProvider()
		config := model.ModelConfig{
			APIKey:   "test-api-key",
			Model:    "gpt-4",
			Endpoint: "http://invalid-endpoint.local",
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
		assert.Empty(t, output.Response)
	})

	t.Run("with files", func(t *testing.T) {
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse request body to verify files are included
			var req ChatCompletionRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			// Should have system, files, and user messages
			assert.True(t, len(req.Messages) >= 3)

			// Check that file content is included
			found := false
			for _, msg := range req.Messages {
				if strings.Contains(msg.Content, "package main") {
					found = true
					break
				}
			}
			assert.True(t, found, "File content should be included in messages")

			// Send response
			response := ChatCompletionResponse{
				ID:    "chatcmpl-123",
				Model: "gpt-4",
				Choices: []Choice{
					{
						Message: ChatMessage{
							Role:    "assistant",
							Content: "I can see the Go code you provided.",
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{TotalTokens: 50},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		// Create model with mock server
		provider := NewProvider()
		config := model.ModelConfig{
			APIKey:   "test-api-key",
			Model:    "gpt-4",
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
	})
}

func TestModel_buildRequest(t *testing.T) {
	provider := NewProvider()
	config := model.ModelConfig{
		APIKey: "test-api-key",
		Model:  "gpt-4",
	}

	modelInstance, err := provider.CreateModel(config)
	require.NoError(t, err)

	openaiModel := modelInstance.(*Model)

	t.Run("basic request", func(t *testing.T) {
		input := model.PromptInput{
			SystemPrompt: "You are helpful.",
			UserPrompt:   "Hello",
			MaxTokens:    1000,
			Temperature:  0.7,
		}

		req := openaiModel.buildRequest(input)

		assert.Equal(t, "gpt-4", req.Model)
		assert.Equal(t, 1000, req.MaxTokens)
		assert.Equal(t, float32(0.7), req.Temperature)
		assert.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "You are helpful.", req.Messages[0].Content)
		assert.Equal(t, "user", req.Messages[1].Role)
		assert.Equal(t, "Hello", req.Messages[1].Content)
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

		req := openaiModel.buildRequest(input)

		assert.Len(t, req.Messages, 3) // system + user + files
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "user", req.Messages[1].Role)
		assert.Equal(t, "Review this", req.Messages[1].Content)
		assert.Equal(t, "user", req.Messages[2].Role)
		assert.Contains(t, req.Messages[2].Content, "test.go")
		assert.Contains(t, req.Messages[2].Content, "package test")
	})

	t.Run("default values", func(t *testing.T) {
		input := model.PromptInput{
			UserPrompt: "Hello",
		}

		req := openaiModel.buildRequest(input)

		assert.Equal(t, 1000, req.MaxTokens)           // default
		assert.Equal(t, float32(0.7), req.Temperature) // default
	})
}

func TestChatMessage_Structure(t *testing.T) {
	msg := ChatMessage{
		Role:    "user",
		Content: "Hello, world!",
	}

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello, world!", msg.Content)
}

func TestChatCompletionRequest_Structure(t *testing.T) {
	req := ChatCompletionRequest{
		Model:       "gpt-4",
		Messages:    []ChatMessage{{Role: "user", Content: "test"}},
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	assert.Equal(t, "gpt-4", req.Model)
	assert.Len(t, req.Messages, 1)
	assert.Equal(t, 1000, req.MaxTokens)
	assert.Equal(t, float32(0.7), req.Temperature)
}

func TestChatCompletionResponse_Structure(t *testing.T) {
	response := ChatCompletionResponse{
		ID:     "test-id",
		Object: "chat.completion",
		Model:  "gpt-4",
		Choices: []Choice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: "Test response",
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			TotalTokens: 100,
		},
	}

	assert.Equal(t, "test-id", response.ID)
	assert.Equal(t, "chat.completion", response.Object)
	assert.Equal(t, "gpt-4", response.Model)
	assert.Len(t, response.Choices, 1)
	assert.Equal(t, "Test response", response.Choices[0].Message.Content)
	assert.Equal(t, 100, response.Usage.TotalTokens)
}
