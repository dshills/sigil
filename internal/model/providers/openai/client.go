// Package openai provides OpenAI API integration for Sigil
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultTimeout = 30 * time.Second
)

// Provider implements the OpenAI model provider
type Provider struct {
	client *http.Client
}

// NewProvider creates a new OpenAI provider
func NewProvider() *Provider {
	return &Provider{
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// CreateModel creates an OpenAI model instance
func (p *Provider) CreateModel(config model.ModelConfig) (model.Model, error) {
	if config.APIKey == "" {
		return nil, errors.ConfigError("CreateModel", "OpenAI API key is required")
	}

	baseURL := config.Endpoint
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := defaultTimeout
	if timeoutVal, ok := config.Options["timeout"].(time.Duration); ok {
		timeout = timeoutVal
	}

	return &Model{
		apiKey:    config.APIKey,
		modelName: config.Model,
		baseURL:   baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// ListModels returns available OpenAI models
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	// Common OpenAI models
	return []string{
		"gpt-4",
		"gpt-4-turbo",
		"gpt-4-turbo-preview",
		"gpt-3.5-turbo",
		"gpt-3.5-turbo-16k",
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "openai"
}

// Model represents an OpenAI model instance
type Model struct {
	apiKey    string
	modelName string
	baseURL   string
	client    *http.Client
}

// RunPrompt executes a prompt against OpenAI API
func (m *Model) RunPrompt(ctx context.Context, input model.PromptInput) (model.PromptOutput, error) {
	start := time.Now()

	// Build request
	req := m.buildRequest(input)

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeModel, "RunPrompt", "failed to marshal request")
	}

	logger.Debug("sending request to OpenAI", "model", m.modelName, "tokens_limit", req.MaxTokens)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeNetwork, "RunPrompt", "failed to create request")
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.apiKey))
	httpReq.Header.Set("User-Agent", "Sigil/1.0")

	// Send request
	resp, err := m.client.Do(httpReq)
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeNetwork, "RunPrompt", "request failed")
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeNetwork, "RunPrompt", "failed to read response")
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return model.PromptOutput{}, errors.New(errors.ErrorTypeNetwork, "RunPrompt",
			fmt.Sprintf("API error %d: %s", resp.StatusCode, string(respBody)))
	}

	// Parse response
	var apiResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeModel, "RunPrompt", "failed to parse response")
	}

	// Extract content
	if len(apiResp.Choices) == 0 {
		return model.PromptOutput{}, errors.New(errors.ErrorTypeModel, "RunPrompt", "no choices in response")
	}

	choice := apiResp.Choices[0]
	content := choice.Message.Content

	// Build output
	output := model.PromptOutput{
		Response:   content,
		TokensUsed: apiResp.Usage.TotalTokens,
		Model:      m.modelName,
		Metadata: map[string]string{
			"finish_reason": choice.FinishReason,
			"model":         apiResp.Model,
		},
	}

	duration := time.Since(start)
	logger.Debug("OpenAI request completed", "duration", duration, "tokens", apiResp.Usage.TotalTokens)

	return output, nil
}

// GetCapabilities returns the model's capabilities
func (m *Model) GetCapabilities() model.ModelCapabilities {
	maxTokens := 4096 // Default for GPT-3.5-turbo

	// Adjust based on model
	switch m.modelName {
	case "gpt-4", "gpt-4-turbo", "gpt-4-turbo-preview":
		maxTokens = 8192
	case "gpt-3.5-turbo-16k":
		maxTokens = 16384
	}

	return model.ModelCapabilities{
		MaxTokens:         maxTokens,
		SupportsImages:    false, // TODO: Add vision model support
		SupportsTools:     true,
		SupportsStreaming: false, // TODO: Add streaming support
	}
}

// Name returns the model identifier
func (m *Model) Name() string {
	return fmt.Sprintf("openai:%s", m.modelName)
}

// buildRequest builds the OpenAI API request
func (m *Model) buildRequest(input model.PromptInput) ChatCompletionRequest {
	messages := []ChatMessage{}

	// Add system message if present
	if input.SystemPrompt != "" {
		messages = append(messages, ChatMessage{
			Role:    "system",
			Content: input.SystemPrompt,
		})
	}

	// Add memory context as messages if present
	for _, memory := range input.Memory {
		messages = append(messages, ChatMessage{
			Role:    "assistant",
			Content: memory.Content,
		})
	}

	// Add user message
	if input.UserPrompt != "" {
		messages = append(messages, ChatMessage{
			Role:    "user",
			Content: input.UserPrompt,
		})
	}

	// Add file context to user message if present
	if len(input.Files) > 0 {
		contextMsg := m.buildFileContext(input.Files)
		if contextMsg != "" {
			messages = append(messages, ChatMessage{
				Role:    "user",
				Content: contextMsg,
			})
		}
	}

	maxTokens := input.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000 // Default
	}

	temperature := float32(0.7) // Default
	if input.Temperature > 0 {
		temperature = float32(input.Temperature)
	}

	return ChatCompletionRequest{
		Model:       m.modelName,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}
}

// buildFileContext builds context from file contents
func (m *Model) buildFileContext(files []model.FileContent) string {
	if len(files) == 0 {
		return ""
	}

	var context bytes.Buffer
	context.WriteString("\nAdditional context files:\n")

	for _, file := range files {
		context.WriteString(fmt.Sprintf("\n--- %s ---\n", file.Path))
		context.WriteString(file.Content)
		context.WriteString("\n")
	}

	return context.String()
}

// OpenAI API types

// ChatCompletionRequest represents a chat completion request
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float32       `json:"temperature,omitempty"`
	TopP        float32       `json:"top_p,omitempty"`
	N           int           `json:"n,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
