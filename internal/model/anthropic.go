package model

import (
	"context"
	"fmt"
	"os"
)

// AnthropicModel implements the Model interface for Anthropic Claude.
type AnthropicModel struct {
	config ModelConfig
	apiKey string
}

// NewAnthropicModel creates a new Anthropic model instance.
func NewAnthropicModel(config ModelConfig) (Model, error) {
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key not provided")
	}

	return &AnthropicModel{
		config: config,
		apiKey: apiKey,
	}, nil
}

// RunPrompt executes a prompt using the Anthropic API.
func (m *AnthropicModel) RunPrompt(ctx context.Context, input PromptInput) (PromptOutput, error) {
	// TODO: Implement Anthropic API call
	return PromptOutput{
		Response:   "Anthropic response placeholder",
		TokensUsed: 0,
		Model:      m.config.Model,
		Metadata:   make(map[string]string),
	}, nil
}

// GetCapabilities returns the model's capabilities.
func (m *AnthropicModel) GetCapabilities() ModelCapabilities {
	return ModelCapabilities{
		MaxTokens:         200000,
		SupportsImages:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	}
}

// Name returns the model provider name.
func (m *AnthropicModel) Name() string {
	return "anthropic"
}
