package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// IntegrationTestConfig holds configuration for integration tests
type IntegrationTestConfig struct {
	ServerCommand string            `json:"command"`
	ServerArgs    []string          `json:"args"`
	ServerEnv     map[string]string `json:"env"`
	TestTimeout   string            `json:"timeout"`
	SkipIfMissing bool              `json:"skipIfMissing"`
}

// loadIntegrationConfig loads integration test configuration
func loadIntegrationConfig() (map[string]IntegrationTestConfig, error) {
	configPath := filepath.Join("testdata", "integration_config.json")

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return default test servers if no config
		return map[string]IntegrationTestConfig{
			"echo-server": {
				ServerCommand: "echo",
				ServerArgs:    []string{`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"tools":true}}}`},
				TestTimeout:   "5s",
				SkipIfMissing: true,
			},
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config map[string]IntegrationTestConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return config, nil
}

// TestIntegration_RealServers tests against real MCP servers
func TestIntegration_RealServers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	config, err := loadIntegrationConfig()
	if err != nil {
		t.Fatalf("Failed to load integration config: %v", err)
	}

	for serverName, serverConfig := range config {
		t.Run(serverName, func(t *testing.T) {
			testRealServer(t, serverName, serverConfig)
		})
	}
}

func testRealServer(t *testing.T, serverName string, config IntegrationTestConfig) {
	// Parse timeout
	timeout := 30 * time.Second
	if config.TestTimeout != "" {
		var err error
		timeout, err = time.ParseDuration(config.TestTimeout)
		if err != nil {
			t.Fatalf("Invalid timeout format: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create process manager
	pm := NewProcessManager()
	defer pm.StopAll()

	// Create server config
	serverConfig := ServerConfig{
		Name:        serverName,
		Command:     config.ServerCommand,
		Args:        config.ServerArgs,
		Env:         config.ServerEnv,
		Transport:   "stdio",
		AutoRestart: false,
		Settings: struct {
			Timeout    string `yaml:"timeout" json:"timeout"`
			MaxRetries int    `yaml:"maxRetries" json:"maxRetries"`
		}{
			Timeout:    "10s",
			MaxRetries: 3,
		},
	}

	// Start server
	server, err := pm.StartServer(ctx, serverConfig)
	if err != nil {
		if config.SkipIfMissing {
			t.Skipf("Server %s not available: %v", serverName, err)
		}
		t.Fatalf("Failed to start server %s: %v", serverName, err)
	}

	// Test basic connectivity
	t.Run("connectivity", func(t *testing.T) {
		testServerConnectivity(t, server)
	})

	// Test protocol operations
	t.Run("protocol", func(t *testing.T) {
		testServerProtocol(t, server)
	})

	// Test error handling
	t.Run("error_handling", func(t *testing.T) {
		testServerErrorHandling(t, server)
	})

	// Test resource operations if supported
	t.Run("resources", func(t *testing.T) {
		testServerResources(t, server)
	})

	// Test tool operations if supported
	t.Run("tools", func(t *testing.T) {
		testServerTools(t, server)
	})
}

func testServerConnectivity(t *testing.T, server *ManagedServer) {
	// Check if server is connected
	if !server.Transport.IsConnected() {
		t.Error("Server should be connected")
	}

	// Check if protocol is initialized
	if !server.Protocol.IsInitialized() {
		t.Error("Protocol should be initialized")
	}

	// Test ping
	response, err := server.Protocol.Ping("connectivity-test")
	if err != nil {
		t.Logf("Ping failed (may not be supported): %v", err)
	} else {
		t.Logf("Ping successful: timestamp=%d, data=%s", response.Timestamp, response.Data)
	}
}

func testServerProtocol(t *testing.T, server *ManagedServer) {
	// Test getting server capabilities
	caps := server.Protocol.GetServerCapabilities()
	if caps == nil {
		t.Error("Server capabilities should not be nil")
		return
	}

	t.Logf("Server capabilities: tools=%v, resources=%v, streaming=%v",
		caps.Tools, caps.Resources, caps.Streaming)
}

func testServerErrorHandling(t *testing.T, server *ManagedServer) {
	// Test invalid method call
	testID := time.Now().UnixNano()
	invalidMsg := &RPCMessage{
		JSONRPC: "2.0",
		ID:      &testID,
		Method:  "invalid/nonexistent/method",
	}

	err := server.Transport.Send(invalidMsg)
	if err != nil {
		t.Logf("Send error (expected for invalid method): %v", err)
	}

	// Give some time for potential response
	time.Sleep(100 * time.Millisecond)
}

func testServerResources(t *testing.T, server *ManagedServer) {
	// Check if server supports resources
	caps := server.Protocol.GetServerCapabilities()
	if caps == nil || !caps.Resources {
		t.Skip("Server does not support resources")
	}

	// Try to list resources
	resources, err := server.Protocol.ListResources()
	if err != nil {
		t.Logf("ListResources failed: %v", err)
		return
	}

	t.Logf("Found %d resources", len(resources))

	// Try to read first resource if available
	if len(resources) > 0 {
		resource := resources[0]
		t.Logf("Testing resource: %s", resource.URI)

		content, err := server.Protocol.ReadResource(resource.URI)
		if err != nil {
			t.Logf("ReadResource failed: %v", err)
		} else {
			contentLen := len(content.Text) + len(content.Blob)
			t.Logf("Resource content length: %d", contentLen)
		}
	}
}

func testServerTools(t *testing.T, server *ManagedServer) {
	// Check if server supports tools
	caps := server.Protocol.GetServerCapabilities()
	if caps == nil || !caps.Tools {
		t.Skip("Server does not support tools")
	}

	// Try to list tools
	tools, err := server.Protocol.ListTools()
	if err != nil {
		t.Logf("ListTools failed: %v", err)
		return
	}

	t.Logf("Found %d tools", len(tools))

	// Try to call first tool if available and it's safe
	if len(tools) > 0 {
		tool := tools[0]
		t.Logf("Testing tool: %s", tool.Name)

		// Only test read-only or safe tools
		if isSafeTool(tool.Name) {
			args := map[string]interface{}{}

			// Add safe default arguments based on tool name
			switch tool.Name {
			case "echo", "ping", "version", "status", "help":
				args["message"] = "test"
			case "list", "ls", "dir":
				args["path"] = "."
			case "read", "cat", "type":
				// Skip file reading tools in tests
				t.Logf("Skipping file reading tool: %s", tool.Name)
				return
			}

			result, err := server.Protocol.CallTool(tool.Name, args)
			if err != nil {
				t.Logf("CallTool failed: %v", err)
			} else {
				t.Logf("Tool result: %d content items, isError=%v",
					len(result.Content), result.IsError)
			}
		} else {
			t.Logf("Skipping potentially unsafe tool: %s", tool.Name)
		}
	}
}

// isSafeTool determines if a tool is safe to call in tests
func isSafeTool(toolName string) bool {
	safeTool := map[string]bool{
		"echo":    true,
		"ping":    true,
		"version": true,
		"status":  true,
		"help":    true,
		"list":    true,
		"ls":      true,
		"dir":     true,
	}

	return safeTool[toolName]
}

// TestIntegration_ProcessManager tests process manager with multiple servers
func TestIntegration_ProcessManager(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	pm := NewProcessManager()
	defer pm.StopAll()

	// Test starting multiple servers
	configs := []ServerConfig{
		{
			Name:      "test-echo-1",
			Command:   "echo",
			Args:      []string{`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"tools":true}}}`},
			Transport: "stdio",
		},
		{
			Name:      "test-echo-2",
			Command:   "echo",
			Args:      []string{`{"jsonrpc":"2.0","id":2,"result":{"capabilities":{"resources":true}}}`},
			Transport: "stdio",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	servers := make([]*ManagedServer, 0, len(configs))

	// Start all servers
	for _, config := range configs {
		server, err := pm.StartServer(ctx, config)
		if err != nil {
			t.Fatalf("Failed to start server %s: %v", config.Name, err)
		}
		servers = append(servers, server)
	}

	// Test server listing
	allServers := pm.ListServers()
	if len(allServers) != len(configs) {
		t.Errorf("Expected %d servers, got %d", len(configs), len(allServers))
	}

	// Test server status
	for _, server := range servers {
		status := server.GetStatus()
		if !status.Connected {
			t.Errorf("Server %s should be connected", server.Name)
		}

		t.Logf("Server %s status: connected=%v, uptime=%v",
			status.Name, status.Connected, status.Uptime)
	}

	// Test connection pooling
	t.Run("connection_pooling", func(t *testing.T) {
		testConnectionPooling(t, pm, configs[0])
	})

	// Test health monitoring
	t.Run("health_monitoring", func(t *testing.T) {
		testHealthMonitoring(t, pm)
	})
}

func testConnectionPooling(t *testing.T, pm *ProcessManager, config ServerConfig) {
	// Get pooled connections
	conn1, err := pm.GetPooledConnection(config.Name)
	if err != nil {
		t.Fatalf("Failed to get pooled connection: %v", err)
	}

	conn2, err := pm.GetPooledConnection(config.Name)
	if err != nil {
		t.Fatalf("Failed to get second pooled connection: %v", err)
	}

	// Connections should be different instances
	if conn1 == conn2 {
		t.Error("Pooled connections should be different instances")
	}

	// Release connections
	pm.ReleaseConnection(conn1)
	pm.ReleaseConnection(conn2)

	// Test pool status
	poolStatus := pm.GetPoolStatus()
	if len(poolStatus) == 0 {
		t.Error("Pool status should not be empty")
	}

	t.Logf("Pool status: %+v", poolStatus)
}

func testHealthMonitoring(t *testing.T, pm *ProcessManager) {
	// Get overall health
	health := pm.GetOverallHealth()
	if health == nil {
		t.Error("Overall health should not be nil")
	}

	// Check expected fields
	expectedFields := []string{"totalServers", "connectedServers", "pooledConnections", "totalRequests"}
	for _, field := range expectedFields {
		if _, exists := health[field]; !exists {
			t.Errorf("Health should include field: %s", field)
		}
	}

	t.Logf("Overall health: %+v", health)
}

// TestIntegration_ErrorRecovery tests error recovery scenarios
func TestIntegration_ErrorRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create a transport that will fail after first connect
	transport := NewStdioTransport("false", []string{}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	// Set up error callback
	var lastError error
	transport.SetErrorCallback(func(err error) {
		lastError = err
	})

	// Configure reconnection
	transport.SetReconnectConfig(2, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to connect (this will fail)
	err := transport.Connect(ctx)
	if err == nil {
		t.Error("Expected connection to fail")
	}

	// Wait a bit for error callback
	time.Sleep(200 * time.Millisecond)

	if lastError == nil {
		t.Error("Error callback should have been called")
	}

	// Check reconnection count
	if transport.GetReconnectCount() == 0 {
		t.Error("Reconnection should have been attempted")
	}

	t.Logf("Reconnection attempts: %d", transport.GetReconnectCount())
	t.Logf("Last error: %v", transport.GetLastError())
}
