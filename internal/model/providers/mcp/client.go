// Package mcp provides Model Context Protocol (MCP) integration for Sigil
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
)

// Provider implements the MCP model provider
type Provider struct {
	processManager *ProcessManager
	configLoader   *ConfigLoader
	serverConfigs  map[string]ServerConfig
	mu             sync.RWMutex
}

// NewProvider creates a new MCP provider
func NewProvider() *Provider {
	globalPath, projectPath := GetDefaultPaths()
	configLoader := NewConfigLoader(globalPath, projectPath)

	provider := &Provider{
		processManager: NewProcessManager(),
		configLoader:   configLoader,
		serverConfigs:  make(map[string]ServerConfig),
	}

	// Load initial configurations
	if err := provider.loadConfigurations(); err != nil {
		logger.Warn("failed to load MCP configurations", "error", err)
	}

	return provider
}

// loadConfigurations loads server configurations from files
func (p *Provider) loadConfigurations() error {
	configs, err := p.configLoader.LoadConfigurations()
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear and reload configurations
	p.serverConfigs = make(map[string]ServerConfig)
	for _, cfg := range configs {
		p.serverConfigs[cfg.Name] = cfg
	}

	logger.Debug("loaded MCP server configurations", "count", len(configs))
	return nil
}

// CreateModel creates an MCP model instance
func (p *Provider) CreateModel(config model.ModelConfig) (model.Model, error) {
	if config.Endpoint == "" {
		return nil, errors.ConfigError("CreateModel", "MCP server endpoint is required")
	}

	// Parse server name and model from endpoint
	// Format: mcp://server-name/model-name
	parts := strings.SplitN(config.Endpoint, "/", 2)
	serverName := strings.TrimPrefix(parts[0], "mcp://")
	modelName := config.Model
	if len(parts) > 1 && modelName == "" {
		modelName = parts[1]
	}

	// Get or start server
	server, err := p.getOrStartServer(serverName, config)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeNetwork, "CreateModel", "failed to get MCP server")
	}

	return &Model{
		modelName: modelName,
		server:    server,
		provider:  p,
	}, nil
}

// ListModels returns available MCP models
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	// List models from all running servers
	servers := p.processManager.ListServers()
	models := make([]string, 0)

	for _, server := range servers {
		if server.Protocol.IsInitialized() {
			caps := server.Protocol.GetServerCapabilities()
			if caps != nil {
				// Add server models
				models = append(models, fmt.Sprintf("mcp://%s/default", server.Name))
			}
		}
	}

	// Add placeholder models for demonstration
	if len(models) == 0 {
		models = []string{
			"mcp://github-mcp/claude-3-sonnet",
			"mcp://postgres-mcp/gpt-4",
			"mcp://custom-server/custom-model",
		}
	}

	return models, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "mcp"
}

// Shutdown stops all MCP servers
func (p *Provider) Shutdown() {
	p.processManager.StopAll()
}

// getOrStartServer gets an existing server or starts a new one
func (p *Provider) getOrStartServer(serverName string, modelConfig model.ModelConfig) (*ManagedServer, error) {
	// Check if server already running
	server, err := p.processManager.GetServer(serverName)
	if err == nil && server.Transport.IsConnected() {
		return server, nil
	}

	// Get server configuration
	var serverConfig ServerConfig

	// First check loaded configurations
	p.mu.RLock()
	loadedConfig, hasLoaded := p.serverConfigs[serverName]
	p.mu.RUnlock()

	if hasLoaded {
		serverConfig = loadedConfig
		// Override with any model-specific options
		p.mergeModelOptions(&serverConfig, modelConfig)
	} else {
		// Parse from model config if not in loaded configs
		serverConfig = p.parseServerConfig(serverName, modelConfig)
	}

	// Start server
	ctx := context.Background()
	return p.processManager.StartServer(ctx, serverConfig)
}

// mergeModelOptions merges model-specific options into server config
func (p *Provider) mergeModelOptions(serverConfig *ServerConfig, modelConfig model.ModelConfig) {
	if modelConfig.Options == nil {
		return
	}

	// Override timeout if specified
	if timeout, ok := modelConfig.Options["timeout"].(string); ok {
		serverConfig.Settings.Timeout = timeout
	}

	// Override environment variables
	if env, ok := modelConfig.Options["env"].(map[string]interface{}); ok {
		if serverConfig.Env == nil {
			serverConfig.Env = make(map[string]string)
		}
		for k, v := range env {
			serverConfig.Env[k] = fmt.Sprint(v)
		}
	}
}

// parseServerConfig creates server configuration from model config
func (p *Provider) parseServerConfig(serverName string, config model.ModelConfig) ServerConfig {
	serverConfig := ServerConfig{
		Name:        serverName,
		Transport:   "stdio",
		AutoRestart: true,
		MaxRestarts: 3,
	}

	// Parse options
	if options := config.Options; options != nil {
		// Command and args
		if cmd, ok := options["command"].(string); ok {
			serverConfig.Command = cmd
		}
		if args, ok := options["args"].([]string); ok {
			serverConfig.Args = args
		} else if argsStr, ok := options["args"].(string); ok {
			serverConfig.Args = strings.Fields(argsStr)
		}

		// Environment
		if env, ok := options["env"].(map[string]string); ok {
			serverConfig.Env = env
		} else if envMap, ok := options["env"].(map[string]interface{}); ok {
			serverConfig.Env = make(map[string]string)
			for k, v := range envMap {
				serverConfig.Env[k] = fmt.Sprint(v)
			}
		}

		// Transport
		if transport, ok := options["transport"].(string); ok {
			serverConfig.Transport = transport
		}

		// Settings
		if timeout, ok := options["timeout"].(string); ok {
			serverConfig.Settings.Timeout = timeout
		} else if timeoutDur, ok := options["timeout"].(time.Duration); ok {
			serverConfig.Settings.Timeout = timeoutDur.String()
		}
		if maxRetries, ok := options["max_retries"].(int); ok {
			serverConfig.Settings.MaxRetries = maxRetries
		}
	}

	// Default command based on server name if not specified
	if serverConfig.Command == "" {
		switch serverName {
		case "github-mcp":
			serverConfig.Command = "npx"
			serverConfig.Args = []string{"-y", "@modelcontextprotocol/server-github"}
		case "postgres-mcp":
			serverConfig.Command = "mcp-server-postgres"
		default:
			serverConfig.Command = serverName
		}
	}

	return serverConfig
}

// Model represents an MCP model instance
type Model struct {
	modelName string
	server    *ManagedServer
	provider  *Provider
}

// RunPrompt executes a prompt against MCP server
func (m *Model) RunPrompt(ctx context.Context, input model.PromptInput) (model.PromptOutput, error) {
	start := time.Now()

	logger.Debug("sending request to MCP server", "model", m.modelName, "server", m.server.Name)

	// Check server is connected
	if !m.server.Transport.IsConnected() {
		return model.PromptOutput{}, errors.New(errors.ErrorTypeNetwork, "RunPrompt", "MCP server not connected")
	}

	// Build completion parameters
	params := CompletionParams{
		Messages: m.buildMessages(input),
		Model:    m.modelName,
		Stream:   false,
	}

	if input.Temperature > 0 {
		params.Temperature = input.Temperature
	}
	if input.MaxTokens > 0 {
		params.MaxTokens = input.MaxTokens
	}

	// Send completion request
	result, err := m.server.Protocol.Complete(params)
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeNetwork, "RunPrompt", "completion request failed")
	}

	// Build output
	output := model.PromptOutput{
		Response:   result.Content,
		TokensUsed: 0,
		Model:      m.modelName,
		Metadata: map[string]string{
			"server":      m.server.Name,
			"mcp_version": "1.0",
		},
	}

	// Add token usage if available
	if result.Usage != nil {
		output.TokensUsed = result.Usage.TotalTokens
		output.Metadata["prompt_tokens"] = fmt.Sprint(result.Usage.PromptTokens)
		output.Metadata["completion_tokens"] = fmt.Sprint(result.Usage.CompletionTokens)
	}

	duration := time.Since(start)
	logger.Debug("MCP request completed", "duration", duration, "tokens", output.TokensUsed)

	return output, nil
}

// GetCapabilities returns the model's capabilities
func (m *Model) GetCapabilities() model.ModelCapabilities {
	// Get capabilities from server if available
	if m.server.Protocol.IsInitialized() {
		caps := m.server.Protocol.GetServerCapabilities()
		if caps != nil {
			return model.ModelCapabilities{
				MaxTokens:         8192, // Default, should query from server
				SupportsImages:    false,
				SupportsTools:     caps.Tools,
				SupportsStreaming: caps.Streaming,
			}
		}
	}

	// Default capabilities
	return model.ModelCapabilities{
		MaxTokens:         4096,
		SupportsImages:    false,
		SupportsTools:     true,
		SupportsStreaming: false,
	}
}

// Name returns the model identifier
func (m *Model) Name() string {
	return fmt.Sprintf("mcp:%s/%s", m.server.Name, m.modelName)
}

// buildMessages converts input to MCP messages
func (m *Model) buildMessages(input model.PromptInput) []Message {
	messages := make([]Message, 0)

	// Add system message
	if input.SystemPrompt != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: input.SystemPrompt,
		})
	}

	// Add memory as assistant messages
	for _, memory := range input.Memory {
		messages = append(messages, Message{
			Role:    "assistant",
			Content: memory.Content,
		})
	}

	// Build user message
	var userContent strings.Builder
	if input.UserPrompt != "" {
		userContent.WriteString(input.UserPrompt)
	}

	// Add file context
	if len(input.Files) > 0 {
		if userContent.Len() > 0 {
			userContent.WriteString("\n\n")
		}
		userContent.WriteString("Additional context files:\n")

		for _, file := range input.Files {
			userContent.WriteString(fmt.Sprintf("\n--- %s ---\n", file.Path))
			userContent.WriteString(file.Content)
			userContent.WriteString("\n")
		}
	}

	if userContent.Len() > 0 {
		messages = append(messages, Message{
			Role:    "user",
			Content: userContent.String(),
		})
	}

	return messages
}

// Tools support for MCP

// ToolCall represents a tool invocation request
type ToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolCallID string          `json:"toolCallId"`
	Content    json.RawMessage `json:"content"`
	IsError    bool            `json:"isError,omitempty"`
}

// CallTool executes a tool on the MCP server
func (m *Model) CallTool(ctx context.Context, toolCall ToolCall) (*ToolResult, error) {
	if !m.server.Protocol.IsInitialized() {
		return nil, fmt.Errorf("server protocol not initialized")
	}

	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal(toolCall.Arguments, &args); err != nil {
		return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Call tool on server
	result, err := m.server.Protocol.CallTool(toolCall.Name, args)
	if err != nil {
		return &ToolResult{
			Content: json.RawMessage(fmt.Sprintf(`{"error": "%s"}`, err.Error())),
			IsError: true,
		}, nil
	}

	// Convert result
	contentBytes, err := json.Marshal(result.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool result: %w", err)
	}

	return &ToolResult{
		Content: contentBytes,
		IsError: result.IsError,
	}, nil
}

// ListTools returns available tools from the server
func (m *Model) ListTools() ([]ToolDefinition, error) {
	if !m.server.Protocol.IsInitialized() {
		return nil, fmt.Errorf("server protocol not initialized")
	}

	return m.server.Protocol.ListTools()
}

// ListResources returns available resources from the server
func (m *Model) ListResources() ([]ResourceDefinition, error) {
	if !m.server.Protocol.IsInitialized() {
		return nil, fmt.Errorf("server protocol not initialized")
	}

	return m.server.Protocol.ListResources()
}

// ReadResource reads a resource from the server
func (m *Model) ReadResource(uri string) (*ResourceContent, error) {
	if !m.server.Protocol.IsInitialized() {
		return nil, fmt.Errorf("server protocol not initialized")
	}

	return m.server.Protocol.ReadResource(uri)
}

// ListPrompts returns available prompt templates from the server
func (m *Model) ListPrompts() ([]PromptTemplate, error) {
	if !m.server.Protocol.IsInitialized() {
		return nil, fmt.Errorf("server protocol not initialized")
	}

	return m.server.Protocol.ListPrompts()
}

// GetPrompt gets a prompt template with arguments
func (m *Model) GetPrompt(name string, arguments map[string]interface{}) (*PromptResult, error) {
	if !m.server.Protocol.IsInitialized() {
		return nil, fmt.Errorf("server protocol not initialized")
	}

	return m.server.Protocol.GetPrompt(name, arguments)
}

// GetConnectionPool returns a pooled connection for this model
func (m *Model) GetConnectionPool() (*ManagedServer, error) {
	return m.provider.processManager.GetPooledConnection(m.server.Name)
}

// ReleaseConnection releases a pooled connection
func (m *Model) ReleaseConnection(server *ManagedServer) {
	m.provider.processManager.ReleaseConnection(server)
}

// GetServerStatus returns the status of the underlying server
func (m *Model) GetServerStatus() ServerStatus {
	return m.server.GetStatus()
}

// Enhanced provider methods

// GetServers returns all managed servers
func (p *Provider) GetServers() []*ManagedServer {
	return p.processManager.ListServers()
}

// GetServerStatus returns status for a specific server
func (p *Provider) GetServerStatus(name string) (ServerStatus, error) {
	server, err := p.processManager.GetServer(name)
	if err != nil {
		return ServerStatus{}, err
	}
	return server.GetStatus(), nil
}

// GetPoolStatus returns connection pool status
func (p *Provider) GetPoolStatus() map[string][]ServerStatus {
	return p.processManager.GetPoolStatus()
}

// GetOverallHealth returns overall health metrics
func (p *Provider) GetOverallHealth() map[string]interface{} {
	return p.processManager.GetOverallHealth()
}

// RestartServer restarts a specific server
func (p *Provider) RestartServer(name string) error {
	ctx := context.Background()
	server, err := p.processManager.GetServer(name)
	if err != nil {
		return err
	}

	// Stop the server
	if err := p.processManager.StopServer(name); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	// Start it again
	_, err = p.processManager.StartServer(ctx, server.Config)
	return err
}

// StartServer starts a server with the given configuration
func (p *Provider) StartServer(config ServerConfig) error {
	ctx := context.Background()
	_, err := p.processManager.StartServer(ctx, config)
	return err
}

// StopServer stops a specific server
func (p *Provider) StopServer(name string) error {
	return p.processManager.StopServer(name)
}

// ReloadConfigurations reloads server configurations from files
func (p *Provider) ReloadConfigurations() error {
	return p.loadConfigurations()
}
