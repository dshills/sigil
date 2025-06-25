package mcp

import (
	"context"
	"time"
)

// Transport defines the interface for MCP communication transports
type Transport interface {
	// Connect establishes the connection
	Connect(ctx context.Context) error

	// Send sends a message
	Send(msg *RPCMessage) error

	// Receive receives the next message
	Receive() (*RPCMessage, error)

	// Close closes the connection
	Close() error

	// IsConnected returns whether the transport is connected
	IsConnected() bool
}

// TransportConfig contains common transport configuration
type TransportConfig struct {
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
	BufferSize int
}

// DefaultTransportConfig returns default transport configuration
func DefaultTransportConfig() TransportConfig {
	return TransportConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: time.Second,
		BufferSize: 4096,
	}
}
