package model

import (
	"context"
	"fmt"
)

// OllamaModel implements the Model interface for Ollama local models
type OllamaModel struct {
	config ModelConfig
}

// NewOllamaModel creates a new Ollama model instance
func NewOllamaModel(config ModelConfig) (Model, error) {
	if config.Model == "" {
		return nil, fmt.Errorf("model name required for Ollama")
	}

	return &OllamaModel{
		config: config,
	}, nil
}

// RunPrompt executes a prompt using Ollama
func (m *OllamaModel) RunPrompt(ctx context.Context, input PromptInput) (PromptOutput, error) {
	// TODO: Implement Ollama API call
	return PromptOutput{
		Response:   "Ollama response placeholder",
		TokensUsed: 0,
		Model:      m.config.Model,
		Metadata:   make(map[string]string),
	}, nil
}

// GetCapabilities returns the model's capabilities
func (m *OllamaModel) GetCapabilities() ModelCapabilities {
	return ModelCapabilities{
		MaxTokens:         8192,
		SupportsImages:    false,
		SupportsTools:     false,
		SupportsStreaming: true,
	}
}

// Name returns the model provider name
func (m *OllamaModel) Name() string {
	return "ollama"
}
