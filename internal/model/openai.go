package model

import (
	"context"
	"fmt"
	"os"
)

// OpenAIModel implements the Model interface for OpenAI
type OpenAIModel struct {
	config ModelConfig
	apiKey string
}

// NewOpenAIModel creates a new OpenAI model instance
func NewOpenAIModel(config ModelConfig) (Model, error) {
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not provided")
	}

	return &OpenAIModel{
		config: config,
		apiKey: apiKey,
	}, nil
}

// RunPrompt executes a prompt using the OpenAI API
func (m *OpenAIModel) RunPrompt(ctx context.Context, input PromptInput) (PromptOutput, error) {
	// TODO: Implement OpenAI API call
	// This is a placeholder implementation
	return PromptOutput{
		Response:   "OpenAI response placeholder",
		TokensUsed: 0,
		Model:      m.config.Model,
		Metadata:   make(map[string]string),
	}, nil
}

// GetCapabilities returns the model's capabilities
func (m *OpenAIModel) GetCapabilities() ModelCapabilities {
	return ModelCapabilities{
		MaxTokens:         4096,
		SupportsImages:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	}
}

// Name returns the model provider name
func (m *OpenAIModel) Name() string {
	return "openai"
}
