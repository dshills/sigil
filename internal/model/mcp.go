package model

import (
	"context"
	"fmt"
)

// MCPModel implements the Model interface for MCP servers
type MCPModel struct {
	config    ModelConfig
	serverURL string
}

// NewMCPModel creates a new MCP model instance
func NewMCPModel(config ModelConfig) (Model, error) {
	serverURL := config.Endpoint
	if serverURL == "" {
		return nil, fmt.Errorf("MCP server URL required")
	}

	return &MCPModel{
		config:    config,
		serverURL: serverURL,
	}, nil
}

// RunPrompt executes a prompt using the MCP server
func (m *MCPModel) RunPrompt(ctx context.Context, input PromptInput) (PromptOutput, error) {
	// TODO: Implement MCP server API call
	return PromptOutput{
		Response:   "MCP response placeholder",
		TokensUsed: 0,
		Model:      m.config.Model,
		Metadata:   make(map[string]string),
	}, nil
}

// GetCapabilities returns the model's capabilities
func (m *MCPModel) GetCapabilities() ModelCapabilities {
	return ModelCapabilities{
		MaxTokens:         8192,
		SupportsImages:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	}
}

// Name returns the model provider name
func (m *MCPModel) Name() string {
	return "mcp"
}
