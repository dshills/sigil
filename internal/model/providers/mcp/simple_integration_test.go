package mcp

import (
	"context"
	"testing"
	"time"
)

// TestSimpleIntegration tests basic MCP functionality with minimal setup
func TestSimpleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("stdio_transport_basic", func(t *testing.T) {
		testStdioTransportBasic(t)
	})

	t.Run("process_manager_lifecycle", func(t *testing.T) {
		testProcessManagerLifecycle(t)
	})

	t.Run("connection_pooling_basic", func(t *testing.T) {
		testConnectionPoolingBasic(t)
	})
}

func testStdioTransportBasic(t *testing.T) {
	// Create a transport with a simple echo command
	transport := NewStdioTransport("echo", []string{`{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"tools":true}}}`}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    2 * time.Second,
	})

	// Disable reconnection for this test
	transport.SetReconnectConfig(0, time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect
	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer transport.Close()

	// Should be connected initially
	if !transport.IsConnected() {
		t.Error("Transport should be connected")
	}

	// Try to receive the output
	msg, err := transport.Receive()
	if err != nil {
		t.Logf("Receive error (expected for echo): %v", err)
	} else {
		if msg.JSONRPC != "2.0" {
			t.Errorf("Expected jsonrpc 2.0, got %s", msg.JSONRPC)
		}
		t.Logf("Received message: %+v", msg)
	}
}

func testProcessManagerLifecycle(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	// Test empty state
	servers := pm.ListServers()
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers initially, got %d", len(servers))
	}

	// Test overall health
	health := pm.GetOverallHealth()
	if health == nil {
		t.Error("Health should not be nil")
	}

	expectedFields := []string{"totalServers", "connectedServers", "pooledConnections", "totalRequests"}
	for _, field := range expectedFields {
		if _, exists := health[field]; !exists {
			t.Errorf("Health missing field: %s", field)
		}
	}

	t.Logf("Initial health: %+v", health)
}

func testConnectionPoolingBasic(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	// Test pool settings
	pm.SetPoolSize(2)

	// Test pool status when empty
	poolStatus := pm.GetPoolStatus()
	if poolStatus == nil {
		t.Error("Pool status should not be nil")
	}

	if len(poolStatus) != 0 {
		t.Errorf("Expected empty pool status, got %d entries", len(poolStatus))
	}

	t.Logf("Pool status: %+v", poolStatus)
}

// TestProtocolHandler tests the protocol handler independently
func TestProtocolHandler(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create a mock transport for testing
	transport := &MockTransport{
		connected: true,
		messages:  make(chan *RPCMessage, 10),
	}

	protocol := NewProtocolHandler(transport)

	// Test initial state
	if protocol.IsInitialized() {
		t.Error("Protocol should not be initialized initially")
	}

	if caps := protocol.GetServerCapabilities(); caps != nil {
		t.Error("Server capabilities should be nil before initialization")
	}

	// Test invalid operations before initialization
	_, err := protocol.Ping("test")
	if err == nil {
		t.Error("Ping should fail before initialization")
	}

	_, err = protocol.ListTools()
	if err == nil {
		t.Error("ListTools should fail before initialization")
	}

	_, err = protocol.ListResources()
	if err == nil {
		t.Error("ListResources should fail before initialization")
	}
}
