// Package mcp provides Model Context Protocol (MCP) integration for Sigil
package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
)

// Provider implements the MCP model provider
type Provider struct {
	// In a real implementation, this would hold MCP client connections
	clients map[string]*MCPClient
}

// NewProvider creates a new MCP provider
func NewProvider() *Provider {
	return &Provider{
		clients: make(map[string]*MCPClient),
	}
}

// CreateModel creates an MCP model instance
func (p *Provider) CreateModel(config model.ModelConfig) (model.Model, error) {
	if config.Endpoint == "" {
		return nil, errors.ConfigError("CreateModel", "MCP server endpoint is required")
	}

	// Parse MCP server configuration
	serverConfig := p.parseServerConfig(config)

	// Create or reuse MCP client
	client, err := p.getOrCreateClient(serverConfig)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeNetwork, "CreateModel", "failed to create MCP client")
	}

	return &Model{
		modelName: config.Model,
		client:    client,
		config:    serverConfig,
	}, nil
}

// ListModels returns available MCP models
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	// In a real implementation, this would query connected MCP servers
	// For now, return common MCP-compatible models
	return []string{
		"mcp://localhost:3000/claude-3-sonnet",
		"mcp://localhost:3000/gpt-4",
		"mcp://localhost:3000/llama-2",
		"mcp://custom-server/custom-model",
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "mcp"
}

// Model represents an MCP model instance
type Model struct {
	modelName string
	client    *MCPClient
	config    *MCPServerConfig
}

// RunPrompt executes a prompt against MCP server
func (m *Model) RunPrompt(ctx context.Context, input model.PromptInput) (model.PromptOutput, error) {
	start := time.Now()

	logger.Debug("sending request to MCP server", "model", m.modelName, "server", m.config.ServerURL)

	// Build MCP request
	request := m.buildRequest(input)

	// Send request to MCP server
	response, err := m.client.SendRequest(ctx, request)
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeNetwork, "RunPrompt", "MCP request failed")
	}

	// Parse response
	if response.Error != nil {
		return model.PromptOutput{}, errors.New(errors.ErrorTypeModel, "RunPrompt",
			fmt.Sprintf("MCP server error: %s", response.Error.Message))
	}

	if response.Result == nil {
		return model.PromptOutput{}, errors.New(errors.ErrorTypeModel, "RunPrompt", "no result in MCP response")
	}

	// Extract content from result
	content, tokensUsed, err := m.parseResult(response.Result)
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeModel, "RunPrompt", "failed to parse MCP result")
	}

	// Build output
	output := model.PromptOutput{
		Response:   content,
		TokensUsed: tokensUsed,
		Model:      m.modelName,
		Metadata: map[string]string{
			"server":      m.config.ServerURL,
			"request_id":  response.ID,
			"mcp_version": "2024-11-05",
		},
	}

	duration := time.Since(start)
	logger.Debug("MCP request completed", "duration", duration, "tokens", tokensUsed)

	return output, nil
}

// GetCapabilities returns the model's capabilities
func (m *Model) GetCapabilities() model.ModelCapabilities {
	// MCP capabilities depend on the underlying model
	// Default to conservative values
	maxTokens := 4096

	// Try to infer from model name
	switch {
	case strings.Contains(m.modelName, "gpt-4"):
		maxTokens = 8192
	case strings.Contains(m.modelName, "claude-3"):
		maxTokens = 200000
	case strings.Contains(m.modelName, "llama"):
		maxTokens = 4096
	}

	return model.ModelCapabilities{
		MaxTokens:         maxTokens,
		SupportsImages:    false, // Depends on MCP server capabilities
		SupportsTools:     true,  // MCP has excellent tool support
		SupportsStreaming: false, // TODO: Implement streaming
	}
}

// Name returns the model identifier
func (m *Model) Name() string {
	return fmt.Sprintf("mcp:%s", m.modelName)
}

// buildRequest builds the MCP request
func (m *Model) buildRequest(input model.PromptInput) *MCPRequest {
	// Build MCP completion request
	request := &MCPRequest{
		JSONRPC: "2.0",
		ID:      generateRequestID(),
		Method:  "completion/complete",
		Params: map[string]interface{}{
			"ref": map[string]interface{}{
				"type": "ref/prompt",
				"name": "default",
			},
			"arguments": map[string]interface{}{
				"model":       m.modelName,
				"temperature": input.Temperature,
				"max_tokens":  input.MaxTokens,
			},
		},
	}

	// Build prompt content
	promptParts := make([]string, 0, 4) // Pre-allocate for system, memory, user, files

	if input.SystemPrompt != "" {
		promptParts = append(promptParts, fmt.Sprintf("System: %s", input.SystemPrompt))
	}

	// Add memory context
	for _, memory := range input.Memory {
		promptParts = append(promptParts, fmt.Sprintf("Assistant: %s", memory.Content))
	}

	if input.UserPrompt != "" {
		promptParts = append(promptParts, fmt.Sprintf("User: %s", input.UserPrompt))
	}

	// Add file context
	if len(input.Files) > 0 {
		contextText := m.buildFileContext(input.Files)
		if contextText != "" {
			promptParts = append(promptParts, contextText)
		}
	}

	// Set the prompt in arguments
	if params, ok := request.Params.(map[string]interface{}); ok {
		if args, ok := params["arguments"].(map[string]interface{}); ok {
			args["prompt"] = strings.Join(promptParts, "\n\n")
		}
	}

	return request
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

// parseResult extracts content and token usage from MCP result
func (m *Model) parseResult(result interface{}) (string, int, error) {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return "", 0, fmt.Errorf("invalid result format")
	}

	// Extract content
	content, ok := resultMap["content"].(string)
	if !ok {
		// Try alternative formats
		if completion, ok := resultMap["completion"].(string); ok {
			content = completion
		} else if text, ok := resultMap["text"].(string); ok {
			content = text
		} else {
			return "", 0, fmt.Errorf("no content found in result")
		}
	}

	// Extract token usage
	tokensUsed := 0
	if usage, ok := resultMap["usage"].(map[string]interface{}); ok {
		if total, ok := usage["total_tokens"].(float64); ok {
			tokensUsed = int(total)
		} else if prompt, ok := usage["prompt_tokens"].(float64); ok {
			if completion, ok := usage["completion_tokens"].(float64); ok {
				tokensUsed = int(prompt + completion)
			}
		}
	}

	return content, tokensUsed, nil
}

// parseServerConfig parses MCP server configuration from model config
func (p *Provider) parseServerConfig(config model.ModelConfig) *MCPServerConfig {
	serverConfig := &MCPServerConfig{
		ServerURL: config.Endpoint,
		Timeout:   30 * time.Second,
	}

	// Parse additional options
	if options := config.Options; options != nil {
		if timeout, ok := options["timeout"].(time.Duration); ok {
			serverConfig.Timeout = timeout
		}
		if transport, ok := options["transport"].(string); ok {
			serverConfig.Transport = transport
		}
		if auth, ok := options["auth"].(map[string]interface{}); ok {
			serverConfig.Auth = auth
		}
	}

	return serverConfig
}

// getOrCreateClient gets or creates an MCP client for the server
func (p *Provider) getOrCreateClient(config *MCPServerConfig) (*MCPClient, error) {
	// Use server URL as key
	key := config.ServerURL

	if client, exists := p.clients[key]; exists {
		return client, nil
	}

	// Create new client
	client, err := NewMCPClient(config)
	if err != nil {
		return nil, err
	}

	p.clients[key] = client
	return client, nil
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("sigil-%d", time.Now().UnixNano())
}

// MCP Protocol Types

// MCPServerConfig holds MCP server configuration
type MCPServerConfig struct {
	ServerURL string
	Transport string // "stdio", "sse", "websocket"
	Timeout   time.Duration
	Auth      map[string]interface{}
}

// MCPClient represents an MCP client connection
type MCPClient struct {
	config *MCPServerConfig
	// In a real implementation, this would hold the actual connection
}

// NewMCPClient creates a new MCP client
func NewMCPClient(config *MCPServerConfig) (*MCPClient, error) {
	// In a real implementation, this would establish the connection
	logger.Debug("creating MCP client", "server", config.ServerURL, "transport", config.Transport)

	return &MCPClient{
		config: config,
	}, nil
}

// SendRequest sends a request to the MCP server
func (c *MCPClient) SendRequest(ctx context.Context, request *MCPRequest) (*MCPResponse, error) {
	// In a real implementation, this would send the request over the configured transport
	logger.Debug("sending MCP request", "method", request.Method, "id", request.ID)

	// Simulate a response for now
	response := &MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result: map[string]interface{}{
			"content": "This is a simulated MCP response. In a real implementation, this would be the actual model output.",
			"usage": map[string]interface{}{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
		},
	}

	return response, nil
}

// MCPRequest represents an MCP JSON-RPC request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents an MCP JSON-RPC response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
