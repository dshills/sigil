package model

import (
	"fmt"
)

// DefaultFactory is the default model factory implementation.
type DefaultFactory struct{}

// NewFactory creates a new model factory.
func NewFactory() Factory {
	return &DefaultFactory{}
}

// CreateModel creates a Model instance based on the configuration.
func (f *DefaultFactory) CreateModel(config ModelConfig) (Model, error) {
	switch config.Provider {
	case "openai":
		return NewOpenAIModel(config)
	case "anthropic":
		return NewAnthropicModel(config)
	case "ollama":
		return NewOllamaModel(config)
	case "mcp":
		return NewMCPModel(config)
	default:
		return nil, fmt.Errorf("unsupported model provider: %s", config.Provider)
	}
}
