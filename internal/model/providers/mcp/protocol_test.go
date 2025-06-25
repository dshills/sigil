package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// MockTransport implements Transport for testing
type MockTransport struct {
	connected bool
	messages  chan *RPCMessage
	responses map[string]*RPCMessage
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		messages:  make(chan *RPCMessage, 10),
		responses: make(map[string]*RPCMessage),
	}
}

func (m *MockTransport) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

func (m *MockTransport) Send(msg *RPCMessage) error {
	if !m.connected {
		return fmt.Errorf("not connected")
	}

	// Simulate response for known methods
	go func() {
		time.Sleep(10 * time.Millisecond) // Simulate network delay

		var response *RPCMessage
		switch msg.Method {
		case "initialize":
			response = &RPCMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: json.RawMessage(`{
					"protocolVersion": "1.0",
					"serverInfo": {
						"name": "test-server",
						"version": "1.0.0"
					},
					"capabilities": {
						"streaming": false,
						"tools": true,
						"resources": true
					}
				}`),
			}
		case "initialized":
			// No response for notifications
			return
		case "completion/complete":
			response = &RPCMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: json.RawMessage(`{
					"content": "Test completion response",
					"usage": {
						"promptTokens": 10,
						"completionTokens": 5,
						"totalTokens": 15
					}
				}`),
			}
		case "shutdown":
			response = &RPCMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result:  json.RawMessage(`null`),
			}
		default:
			response = &RPCMessage{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Error: &RPCError{
					Code:    MethodNotFound,
					Message: "Method not found",
				},
			}
		}

		m.messages <- response
	}()

	return nil
}

func (m *MockTransport) Receive() (*RPCMessage, error) {
	if !m.connected {
		return nil, fmt.Errorf("not connected")
	}

	select {
	case msg := <-m.messages:
		return msg, nil
	case <-time.After(1 * time.Second):
		return nil, fmt.Errorf("timeout")
	}
}

func (m *MockTransport) Close() error {
	m.connected = false
	close(m.messages)
	return nil
}

func (m *MockTransport) IsConnected() bool {
	return m.connected
}

// Tests

func TestProtocolInitialize(t *testing.T) {
	// Create mock transport
	transport := NewMockTransport()
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Failed to connect transport: %v", err)
	}
	defer transport.Close()

	// Create protocol handler
	handler := NewProtocolHandler(transport)

	// Start message processor
	go func() {
		for transport.IsConnected() {
			msg, err := transport.Receive()
			if err != nil {
				return
			}
			handler.ProcessMessage(msg)
		}
	}()

	// Test initialization
	clientInfo := ClientInfo{
		Name:    "test-client",
		Version: "1.0.0",
	}

	capabilities := ClientCapabilities{
		Streaming: false,
		Tools:     true,
		Resources: true,
	}

	result, err := handler.Initialize(clientInfo, capabilities)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify result
	if result.ProtocolVersion != "1.0" {
		t.Errorf("Expected protocol version 1.0, got %s", result.ProtocolVersion)
	}

	if result.ServerInfo.Name != "test-server" {
		t.Errorf("Expected server name test-server, got %s", result.ServerInfo.Name)
	}

	if !result.Capabilities.Tools {
		t.Error("Expected tools capability to be true")
	}

	if !handler.IsInitialized() {
		t.Error("Handler should be initialized")
	}
}

func TestProtocolComplete(t *testing.T) {
	// Create and initialize protocol
	transport := NewMockTransport()
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Failed to connect transport: %v", err)
	}
	defer transport.Close()

	handler := NewProtocolHandler(transport)

	// Start message processor
	go func() {
		for transport.IsConnected() {
			msg, err := transport.Receive()
			if err != nil {
				return
			}
			handler.ProcessMessage(msg)
		}
	}()

	// Initialize first
	_, err := handler.Initialize(
		ClientInfo{Name: "test", Version: "1.0"},
		ClientCapabilities{Tools: true},
	)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test completion
	params := CompletionParams{
		Messages: []Message{
			{Role: "user", Content: "Hello, world!"},
		},
		Model: "test-model",
	}

	result, err := handler.Complete(params)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if result.Content != "Test completion response" {
		t.Errorf("Expected test response, got %s", result.Content)
	}

	if result.Usage == nil || result.Usage.TotalTokens != 15 {
		t.Error("Unexpected usage data")
	}
}

func TestProtocolShutdown(t *testing.T) {
	// Create and initialize protocol
	transport := NewMockTransport()
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Failed to connect transport: %v", err)
	}

	handler := NewProtocolHandler(transport)

	// Start message processor
	go func() {
		for transport.IsConnected() {
			msg, err := transport.Receive()
			if err != nil {
				return
			}
			handler.ProcessMessage(msg)
		}
	}()

	// Initialize
	_, err := handler.Initialize(
		ClientInfo{Name: "test", Version: "1.0"},
		ClientCapabilities{},
	)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test shutdown
	err = handler.Shutdown()
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	if handler.IsInitialized() {
		t.Error("Handler should not be initialized after shutdown")
	}

	if transport.IsConnected() {
		t.Error("Transport should be closed after shutdown")
	}
}
