package mcp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStdioTransport_ErrorCallback(t *testing.T) {
	transport := NewStdioTransport("echo", []string{"test"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	var callbackError error
	var callbackCalled bool

	transport.SetErrorCallback(func(err error) {
		callbackError = err
		callbackCalled = true
	})

	// Simulate an error by setting lastError
	transport.mu.Lock()
	transport.lastError = errors.New("test error")
	transport.mu.Unlock()

	// Trigger error callback
	if transport.errorCallback != nil {
		transport.errorCallback(transport.lastError)
	}

	if !callbackCalled {
		t.Error("Error callback was not called")
	}

	if callbackError == nil || callbackError.Error() != "test error" {
		t.Errorf("Expected error 'test error', got %v", callbackError)
	}
}

func TestStdioTransport_ReconnectConfig(t *testing.T) {
	transport := NewStdioTransport("echo", []string{"test"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	// Test default values
	if transport.maxReconnects != 3 {
		t.Errorf("Expected default maxReconnects to be 3, got %d", transport.maxReconnects)
	}

	if transport.reconnectDelay != 2*time.Second {
		t.Errorf("Expected default reconnectDelay to be 2s, got %v", transport.reconnectDelay)
	}

	// Test setting custom values
	transport.SetReconnectConfig(5, 5*time.Second)

	if transport.maxReconnects != 5 {
		t.Errorf("Expected maxReconnects to be 5, got %d", transport.maxReconnects)
	}

	if transport.reconnectDelay != 5*time.Second {
		t.Errorf("Expected reconnectDelay to be 5s, got %v", transport.reconnectDelay)
	}
}

func TestStdioTransport_GetLastError(t *testing.T) {
	transport := NewStdioTransport("echo", []string{"test"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	// Initially no error
	if err := transport.GetLastError(); err != nil {
		t.Errorf("Expected no error initially, got %v", err)
	}

	// Set an error
	testErr := errors.New("test error")
	transport.mu.Lock()
	transport.lastError = testErr
	transport.mu.Unlock()

	// Get the error
	if err := transport.GetLastError(); err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}
}

func TestStdioTransport_GetReconnectCount(t *testing.T) {
	transport := NewStdioTransport("echo", []string{"test"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	// Initially zero
	if count := transport.GetReconnectCount(); count != 0 {
		t.Errorf("Expected reconnect count to be 0, got %d", count)
	}

	// Increment reconnect count
	transport.mu.Lock()
	transport.reconnectCount = 3
	transport.mu.Unlock()

	// Check the count
	if count := transport.GetReconnectCount(); count != 3 {
		t.Errorf("Expected reconnect count to be 3, got %d", count)
	}
}

func TestStdioTransport_CloseResetsState(t *testing.T) {
	transport := NewStdioTransport("echo", []string{"test"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	// Set some error state
	transport.mu.Lock()
	transport.reconnectCount = 5
	transport.lastError = errors.New("some error")
	transport.connected = true
	transport.mu.Unlock()

	// Close should reset state
	err := transport.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Check that state was reset
	if count := transport.GetReconnectCount(); count != 0 {
		t.Errorf("Expected reconnect count to be reset to 0, got %d", count)
	}

	if err := transport.GetLastError(); err != nil {
		t.Errorf("Expected last error to be reset to nil, got %v", err)
	}

	if transport.IsConnected() {
		t.Error("Expected transport to be disconnected after close")
	}
}

func TestStdioTransport_MessageHandler(t *testing.T) {
	transport := NewStdioTransport("echo", []string{"test"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	var handledMessage *RPCMessage
	var handlerCalled bool

	transport.SetMessageHandler(func(msg *RPCMessage) {
		handledMessage = msg
		handlerCalled = true
	})

	// Create a test message
	testID := int64(123)
	testMessage := &RPCMessage{
		ID:      &testID,
		Method:  "test-method",
		JSONRPC: "2.0",
	}

	// Simulate message handling
	if transport.messageHandler != nil {
		transport.messageHandler(testMessage)
	}

	if !handlerCalled {
		t.Error("Message handler was not called")
	}

	if handledMessage != testMessage {
		t.Error("Message handler received wrong message")
	}
}

func TestStdioTransport_ConnectTwice(t *testing.T) {
	transport := NewStdioTransport("echo", []string{"test"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	ctx := context.Background()

	// First connect should work
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("First connect failed: %v", err)
	}

	if !transport.IsConnected() {
		t.Error("Transport should be connected after first connect")
	}

	// Second connect should be a no-op
	if err := transport.Connect(ctx); err != nil {
		t.Errorf("Second connect failed: %v", err)
	}

	if !transport.IsConnected() {
		t.Error("Transport should still be connected after second connect")
	}

	// Clean up
	transport.Close()
}

func TestStdioTransport_SendWhenDisconnected(t *testing.T) {
	transport := NewStdioTransport("echo", []string{"test"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	// Try to send without connecting
	testID := int64(456)
	testMessage := &RPCMessage{
		ID:      &testID,
		Method:  "test-method",
		JSONRPC: "2.0",
	}

	err := transport.Send(testMessage)
	if err == nil {
		t.Error("Expected error when sending on disconnected transport")
	}

	if err.Error() != "transport not connected" {
		t.Errorf("Expected 'transport not connected' error, got: %v", err)
	}
}

func TestStdioTransport_ReceiveWhenDisconnected(t *testing.T) {
	transport := NewStdioTransport("echo", []string{"test"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	// Try to receive without connecting
	_, err := transport.Receive()
	if err == nil {
		t.Error("Expected error when receiving on disconnected transport")
	}

	if err.Error() != "transport not connected" {
		t.Errorf("Expected 'transport not connected' error, got: %v", err)
	}
}

func TestStdioTransport_CloseIdempotent(t *testing.T) {
	transport := NewStdioTransport("echo", []string{"test"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    time.Second,
	})

	// Close should work even when not connected
	err := transport.Close()
	if err != nil {
		t.Errorf("Close on disconnected transport failed: %v", err)
	}

	// Second close should also work
	err = transport.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

// TestStdioTransport_Integration tests the transport with a real echo command
func TestStdioTransport_Integration(t *testing.T) {
	// Skip if we don't have echo command
	transport := NewStdioTransport("echo", []string{"{\"jsonrpc\":\"2.0\",\"id\":123,\"result\":\"ok\"}"}, nil, TransportConfig{
		BufferSize: 1024,
		Timeout:    5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Connect
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// The echo command will output the JSON and exit
	// We should be able to receive the message
	msg, err := transport.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if msg.ID == nil || *msg.ID != 123 {
		t.Errorf("Expected message ID to be 123, got %v", msg.ID)
	}

	if msg.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc '2.0', got %v", msg.JSONRPC)
	}
}
