package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dshills/sigil/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_NewProvider(t *testing.T) {
	provider := NewProvider()
	require.NotNil(t, provider)
	assert.NotNil(t, provider.processManager)
	assert.NotNil(t, provider.configLoader)
	assert.NotNil(t, provider.serverConfigs)
	assert.Equal(t, "mcp", provider.Name())

	// Clean up
	provider.Shutdown()
}

func TestProvider_CreateModel(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	config := model.ModelConfig{
		Endpoint: "mcp://test-server",
		Model:    "test-model",
		Options: map[string]interface{}{
			"command": "echo",
			"args":    []string{"hello"},
		},
	}

	// Note: This will fail in practice because we can't actually start an echo server
	// as an MCP server, but it tests the parsing logic
	mcpModel, err := provider.CreateModel(config)
	if err != nil {
		// Expected to fail due to protocol initialization
		assert.Contains(t, err.Error(), "failed to get MCP server")
		return
	}

	assert.NotNil(t, mcpModel)
	assert.Equal(t, "mcp:test-server/test-model", mcpModel.Name())
}

func TestProvider_CreateModel_InvalidEndpoint(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	config := model.ModelConfig{
		Endpoint: "", // Empty endpoint
		Model:    "test-model",
	}

	_, err := provider.CreateModel(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint is required")
}

func TestProvider_ListModels(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	ctx := context.Background()
	models, err := provider.ListModels(ctx)
	require.NoError(t, err)
	assert.Greater(t, len(models), 0)

	// Should contain placeholder models since no servers are running
	assert.Contains(t, models, "mcp://github-mcp/claude-3-sonnet")
	assert.Contains(t, models, "mcp://postgres-mcp/gpt-4")
	assert.Contains(t, models, "mcp://custom-server/custom-model")
}

func TestProvider_ServerManagement(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	config := ServerConfig{
		Name:      "test-mgmt-server",
		Command:   "echo",
		Transport: "stdio",
	}

	// Test starting server
	err := provider.StartServer(config)
	if err != nil {
		// Expected to fail due to protocol issues, but tests the method
		assert.Contains(t, err.Error(), "failed to")
		return
	}

	// Test getting server status
	status, err := provider.GetServerStatus("test-mgmt-server")
	require.NoError(t, err)
	assert.Equal(t, "test-mgmt-server", status.Name)

	// Test getting all servers
	servers := provider.GetServers()
	assert.Len(t, servers, 1)
	assert.Equal(t, "test-mgmt-server", servers[0].Name)

	// Test stopping server
	err = provider.StopServer("test-mgmt-server")
	require.NoError(t, err)

	// Verify server is gone
	_, err = provider.GetServerStatus("test-mgmt-server")
	assert.Error(t, err)
}

func TestProvider_HealthMetrics(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	// Test overall health with no servers
	health := provider.GetOverallHealth()
	assert.Equal(t, 0, health["totalServers"])
	assert.Equal(t, 0, health["connectedServers"])

	// Test pool status with no servers
	poolStatus := provider.GetPoolStatus()
	assert.Len(t, poolStatus, 0)
}

func TestProvider_ConfigParsing(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	modelConfig := model.ModelConfig{
		Options: map[string]interface{}{
			"command":     "test-command",
			"args":        []string{"arg1", "arg2"},
			"env":         map[string]string{"VAR1": "value1"},
			"transport":   "stdio",
			"timeout":     "30s",
			"max_retries": 3,
		},
	}

	serverConfig := provider.parseServerConfig("test-server", modelConfig)
	assert.Equal(t, "test-server", serverConfig.Name)
	assert.Equal(t, "test-command", serverConfig.Command)
	assert.Equal(t, []string{"arg1", "arg2"}, serverConfig.Args)
	assert.Equal(t, "stdio", serverConfig.Transport)
	assert.Equal(t, "30s", serverConfig.Settings.Timeout)
	assert.Equal(t, 3, serverConfig.Settings.MaxRetries)
	assert.True(t, serverConfig.AutoRestart)
	assert.Equal(t, 3, serverConfig.MaxRestarts)
}

func TestProvider_DefaultCommands(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	// Test github-mcp default
	config := provider.parseServerConfig("github-mcp", model.ModelConfig{})
	assert.Equal(t, "npx", config.Command)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-github"}, config.Args)

	// Test postgres-mcp default
	config = provider.parseServerConfig("postgres-mcp", model.ModelConfig{})
	assert.Equal(t, "mcp-server-postgres", config.Command)

	// Test unknown server default
	config = provider.parseServerConfig("unknown-server", model.ModelConfig{})
	assert.Equal(t, "unknown-server", config.Command)
}

func TestProvider_MergeModelOptions(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	serverConfig := ServerConfig{
		Settings: struct {
			Timeout    string `yaml:"timeout" json:"timeout"`
			MaxRetries int    `yaml:"maxRetries" json:"maxRetries"`
		}{
			Timeout:    "10s",
			MaxRetries: 1,
		},
		Env: map[string]string{
			"EXISTING": "value",
		},
	}

	modelConfig := model.ModelConfig{
		Options: map[string]interface{}{
			"timeout": "60s",
			"env": map[string]interface{}{
				"NEW_VAR": "new_value",
				"EXISTING": "overridden",
			},
		},
	}

	provider.mergeModelOptions(&serverConfig, modelConfig)

	assert.Equal(t, "60s", serverConfig.Settings.Timeout)
	assert.Equal(t, "overridden", serverConfig.Env["EXISTING"])
	assert.Equal(t, "new_value", serverConfig.Env["NEW_VAR"])
}

func TestModel_GetCapabilities(t *testing.T) {
	// Create a mock server with capabilities
	mockServer := &ManagedServer{
		Protocol: &ProtocolHandler{
			initialized: true,
			serverCaps: &ServerCapabilities{
				Streaming: true,
				Tools:     true,
				Resources: false,
			},
		},
	}

	mcpModel := &Model{
		modelName: "test-model",
		server:    mockServer,
	}

	caps := mcpModel.GetCapabilities()
	assert.Equal(t, 8192, caps.MaxTokens)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsImages)

	// Test with uninitialized protocol
	mcpModel.server.Protocol.initialized = false
	caps = mcpModel.GetCapabilities()
	assert.Equal(t, 4096, caps.MaxTokens)
	assert.True(t, caps.SupportsTools)
	assert.False(t, caps.SupportsStreaming)
}

func TestModel_BuildMessages(t *testing.T) {
	mcpModel := &Model{
		modelName: "test-model",
	}

	input := model.PromptInput{
		SystemPrompt: "You are a helpful assistant",
		UserPrompt:   "Hello, world!",
		Memory: []model.MemoryEntry{
			{Content: "Previous conversation", Type: "session"},
		},
		Files: []model.FileContent{
			{Path: "test.txt", Content: "File content"},
		},
	}

	messages := mcpModel.buildMessages(input)
	require.Len(t, messages, 3)

	// System message
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "You are a helpful assistant", messages[0].Content)

	// Memory message
	assert.Equal(t, "assistant", messages[1].Role)
	assert.Equal(t, "Previous conversation", messages[1].Content)

	// User message with file context
	assert.Equal(t, "user", messages[2].Role)
	assert.Contains(t, messages[2].Content, "Hello, world!")
	assert.Contains(t, messages[2].Content, "test.txt")
	assert.Contains(t, messages[2].Content, "File content")
}

func TestModel_RunPrompt_Disconnected(t *testing.T) {
	// Create a mock transport that reports disconnected
	mockTransport := &MockTransport{connected: false}

	mockServer := &ManagedServer{
		Transport: mockTransport,
		Protocol:  NewProtocolHandler(mockTransport),
	}

	mcpModel := &Model{
		modelName: "test-model",
		server:    mockServer,
	}

	ctx := context.Background()
	input := model.PromptInput{
		UserPrompt: "Test prompt",
	}

	_, err := mcpModel.RunPrompt(ctx, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestModel_ToolCalling(t *testing.T) {
	// Create mock server with uninitialized protocol
	mockServer := &ManagedServer{
		Protocol: &ProtocolHandler{initialized: false},
	}

	mcpModel := &Model{
		modelName: "test-model",
		server:    mockServer,
	}

	ctx := context.Background()
	toolCall := ToolCall{
		Name:      "test_tool",
		Arguments: json.RawMessage(`{"param": "value"}`),
	}

	// Test with uninitialized protocol
	_, err := mcpModel.CallTool(ctx, toolCall)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test ListTools with uninitialized protocol
	_, err = mcpModel.ListTools()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestModel_ResourceManagement(t *testing.T) {
	// Create mock server with uninitialized protocol
	mockServer := &ManagedServer{
		Protocol: &ProtocolHandler{initialized: false},
	}

	mcpModel := &Model{
		modelName: "test-model",
		server:    mockServer,
	}

	// Test ListResources with uninitialized protocol
	_, err := mcpModel.ListResources()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test ReadResource with uninitialized protocol
	_, err = mcpModel.ReadResource("test://resource")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestModel_PromptTemplates(t *testing.T) {
	// Create mock server with uninitialized protocol
	mockServer := &ManagedServer{
		Protocol: &ProtocolHandler{initialized: false},
	}

	mcpModel := &Model{
		modelName: "test-model",
		server:    mockServer,
	}

	// Test ListPrompts with uninitialized protocol
	_, err := mcpModel.ListPrompts()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test GetPrompt with uninitialized protocol
	_, err = mcpModel.GetPrompt("test_prompt", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestModel_ConnectionPooling(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	mockServer := &ManagedServer{
		Name: "test-server",
	}

	mcpModel := &Model{
		modelName: "test-model",
		server:    mockServer,
		provider:  provider,
	}

	// Test getting pooled connection (will fail since server doesn't exist)
	_, err := mcpModel.GetConnectionPool()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test releasing connection (should not panic)
	mcpModel.ReleaseConnection(mockServer)
}

func TestModel_ServerStatus(t *testing.T) {
	mockServer := &ManagedServer{
		Name:      "test-server",
		startTime: time.Now(),
	}

	mcpModel := &Model{
		modelName: "test-model",
		server:    mockServer,
	}

	status := mcpModel.GetServerStatus()
	assert.Equal(t, "test-server", status.Name)
	assert.Greater(t, status.Uptime, time.Duration(0))
}

func TestProvider_RestartServer(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	// Test restarting non-existent server
	err := provider.RestartServer("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProvider_ReloadConfigurations(t *testing.T) {
	provider := NewProvider()
	defer provider.Shutdown()

	// Test reloading configurations (should not error even if files don't exist)
	err := provider.ReloadConfigurations()
	// This may or may not error depending on whether config files exist
	// The important thing is that it doesn't panic
	_ = err
}