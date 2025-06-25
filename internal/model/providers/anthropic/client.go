// Package anthropic provides Anthropic Claude API integration for Sigil
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
)

const (
	defaultBaseURL = "https://api.anthropic.com/v1"
	defaultTimeout = 30 * time.Second
	apiVersion     = "2023-06-01"
)

// Provider implements the Anthropic model provider
type Provider struct {
	client *http.Client
}

// NewProvider creates a new Anthropic provider
func NewProvider() *Provider {
	return &Provider{
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// CreateModel creates an Anthropic model instance
func (p *Provider) CreateModel(config model.ModelConfig) (model.Model, error) {
	if config.APIKey == "" {
		return nil, errors.ConfigError("CreateModel", "Anthropic API key is required")
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

// ListModels returns available Anthropic models
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	// Common Anthropic models
	return []string{
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
		"claude-2.1",
		"claude-2.0",
		"claude-instant-1.2",
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "anthropic"
}

// Model represents an Anthropic model instance
type Model struct {
	apiKey    string
	modelName string
	baseURL   string
	client    *http.Client
}

// RunPrompt executes a prompt against Anthropic API
func (m *Model) RunPrompt(ctx context.Context, input model.PromptInput) (model.PromptOutput, error) {
	start := time.Now()

	// Build request
	req := m.buildRequest(input)

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeModel, "RunPrompt", "failed to marshal request")
	}

	logger.Debug("sending request to Anthropic", "model", m.modelName, "max_tokens", req.MaxTokens)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/messages", bytes.NewReader(reqBody))
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeNetwork, "RunPrompt", "failed to create request")
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", m.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)
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
	var apiResp MessageResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeModel, "RunPrompt", "failed to parse response")
	}

	// Extract content
	if len(apiResp.Content) == 0 {
		return model.PromptOutput{}, errors.New(errors.ErrorTypeModel, "RunPrompt", "no content in response")
	}

	var content strings.Builder
	for _, c := range apiResp.Content {
		if c.Type == "text" {
			content.WriteString(c.Text)
		}
	}

	// Build output
	output := model.PromptOutput{
		Response:   content.String(),
		TokensUsed: apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
		Model:      m.modelName,
		Metadata: map[string]string{
			"stop_reason": apiResp.StopReason,
			"model":       apiResp.Model,
			"id":          apiResp.ID,
		},
	}

	duration := time.Since(start)
	logger.Debug("Anthropic request completed", "duration", duration, "tokens", output.TokensUsed)

	return output, nil
}

// GetCapabilities returns the model's capabilities
func (m *Model) GetCapabilities() model.ModelCapabilities {
	maxTokens := 100000 // Default for Claude-3

	// Adjust based on model
	switch {
	case strings.Contains(m.modelName, "claude-3"):
		maxTokens = 200000
	case strings.Contains(m.modelName, "claude-2"):
		maxTokens = 100000
	case strings.Contains(m.modelName, "claude-instant"):
		maxTokens = 100000
	}

	return model.ModelCapabilities{
		MaxTokens:         maxTokens,
		SupportsImages:    strings.Contains(m.modelName, "claude-3"), // Claude-3 models support vision
		SupportsTools:     true,
		SupportsStreaming: false, // Streaming support could be added in future
	}
}

// Name returns the model identifier
func (m *Model) Name() string {
	return fmt.Sprintf("anthropic:%s", m.modelName)
}

// buildRequest builds the Anthropic API request
func (m *Model) buildRequest(input model.PromptInput) MessageRequest {
	messages := []Message{}

	// Add memory context as messages if present
	for _, memory := range input.Memory {
		messages = append(messages, Message{
			Role: "assistant",
			Content: []ContentBlock{{
				Type: "text",
				Text: memory.Content,
			}},
		})
	}

	// Build user message content
	userContent := []ContentBlock{}

	if input.UserPrompt != "" {
		userContent = append(userContent, ContentBlock{
			Type: "text",
			Text: input.UserPrompt,
		})
	}

	// Add file context
	if len(input.Files) > 0 {
		contextText := m.buildFileContext(input.Files)
		if contextText != "" {
			userContent = append(userContent, ContentBlock{
				Type: "text",
				Text: contextText,
			})
		}
	}

	// Add user message if we have content
	if len(userContent) > 0 {
		messages = append(messages, Message{
			Role:    "user",
			Content: userContent,
		})
	}

	maxTokens := input.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000 // Default
	}

	temperature := float32(0.7) // Default
	if input.Temperature > 0 {
		temperature = float32(input.Temperature)
	}

	req := MessageRequest{
		Model:       m.modelName,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	// Add system prompt if present
	if input.SystemPrompt != "" {
		req.System = input.SystemPrompt
	}

	return req
}

// buildFileContext builds context from file contents
func (m *Model) buildFileContext(files []model.FileContent) string {
	if len(files) == 0 {
		return ""
	}

	var context strings.Builder
	context.WriteString("\nAdditional context files:\n")

	for _, file := range files {
		context.WriteString(fmt.Sprintf("\n--- %s ---\n", file.Path))
		context.WriteString(file.Content)
		context.WriteString("\n")
	}

	return context.String()
}

// Anthropic API types

// MessageRequest represents a message request
type MessageRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float32   `json:"temperature,omitempty"`
	System      string    `json:"system,omitempty"`
}

// Message represents a conversation message
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a content block
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MessageResponse represents a message response
type MessageResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
