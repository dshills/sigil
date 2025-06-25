package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/dshills/sigil/internal/logger"
)

// StdioTransport implements Transport for stdio-based MCP servers
type StdioTransport struct {
	command string
	args    []string
	env     []string
	config  TransportConfig

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	reader *bufio.Reader
	writer *bufio.Writer

	mu        sync.RWMutex
	connected bool
	ctx       context.Context
	cancel    context.CancelFunc
	parentCtx context.Context // Store parent context for reconnection

	messageHandler  func(*RPCMessage)
	reconnectCount  int
	maxReconnects   int
	lastError       error
	errorCallback   func(error)
	reconnectDelay  time.Duration
	reconnectActive bool // Prevent concurrent reconnection attempts
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(command string, args []string, env []string, config TransportConfig) *StdioTransport {
	return &StdioTransport{
		command:        command,
		args:           args,
		env:            env,
		config:         config,
		maxReconnects:  3,
		reconnectDelay: 2 * time.Second,
	}
}

// SetMessageHandler sets the handler for incoming messages
func (t *StdioTransport) SetMessageHandler(handler func(*RPCMessage)) {
	t.messageHandler = handler
}

// SetErrorCallback sets the callback for connection errors
func (t *StdioTransport) SetErrorCallback(callback func(error)) {
	t.errorCallback = callback
}

// SetReconnectConfig configures reconnection behavior
func (t *StdioTransport) SetReconnectConfig(maxReconnects int, delay time.Duration) {
	t.maxReconnects = maxReconnects
	t.reconnectDelay = delay
}

// GetLastError returns the last error encountered
func (t *StdioTransport) GetLastError() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastError
}

// GetReconnectCount returns the current reconnection attempt count
func (t *StdioTransport) GetReconnectCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.reconnectCount
}

// Connect starts the MCP server process
func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	// Store parent context for reconnection
	t.parentCtx = ctx

	// Create cancellable context
	t.ctx, t.cancel = context.WithCancel(ctx)

	// Create command
	t.cmd = exec.CommandContext(t.ctx, t.command, t.args...)
	if len(t.env) > 0 {
		t.cmd.Env = append(t.cmd.Env, t.env...)
	}

	// Get pipes
	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	t.stderr, err = t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the process
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Create buffered readers/writers
	t.reader = bufio.NewReaderSize(t.stdout, t.config.BufferSize)
	t.writer = bufio.NewWriterSize(t.stdin, t.config.BufferSize)

	t.connected = true

	// Start reading messages in background
	go t.readLoop()

	// Start error reader
	go t.readErrors()

	return nil
}

// Send sends a message to the server
func (t *StdioTransport) Send(msg *RPCMessage) error {
	t.mu.RLock()
	if !t.connected {
		t.mu.RUnlock()
		return fmt.Errorf("transport not connected")
	}
	writer := t.writer
	t.mu.RUnlock()

	// Encode message as JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write message followed by newline
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Flush the writer
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

// Receive receives the next message (blocking)
func (t *StdioTransport) Receive() (*RPCMessage, error) {
	t.mu.RLock()
	if !t.connected {
		t.mu.RUnlock()
		return nil, fmt.Errorf("transport not connected")
	}
	reader := t.reader
	t.mu.RUnlock()

	// Read line
	line, err := reader.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("server closed connection")
		}
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	// Parse JSON
	var msg RPCMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	return &msg, nil
}

// Close closes the connection and stops the process
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	t.connected = false

	// Reset reconnection state
	t.reconnectCount = 0
	t.lastError = nil
	t.reconnectActive = false

	// Cancel context to stop readers
	if t.cancel != nil {
		t.cancel()
	}

	// Close pipes
	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.stdout != nil {
		t.stdout.Close()
	}
	if t.stderr != nil {
		t.stderr.Close()
	}

	// Wait for process to exit (with timeout)
	if t.cmd != nil {
		done := make(chan error, 1)
		go func() {
			done <- t.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited normally
		case <-time.After(5 * time.Second):
			// Force kill after timeout
			if t.cmd.Process != nil {
				t.cmd.Process.Kill()
				<-done
			}
		}
	}

	return nil
}

// IsConnected returns whether the transport is connected
func (t *StdioTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// readLoop continuously reads messages from stdout
func (t *StdioTransport) readLoop() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			msg, err := t.Receive()
			if err != nil {
				// Check if we're still supposed to be connected
				t.mu.RLock()
				connected := t.connected
				t.mu.RUnlock()

				if connected {
					// Handle read error with reconnection logic
					t.mu.Lock()
					t.lastError = err
					t.mu.Unlock()

					logger.Warn("MCP transport read error", "error", err, "reconnect_count", t.reconnectCount)

					// Notify error callback
					if t.errorCallback != nil {
						t.errorCallback(err)
					}

					// Attempt reconnection if within limits and not already reconnecting
					if t.reconnectCount < t.maxReconnects {
						go t.attemptReconnect()
					} else {
						logger.Error("MCP transport max reconnects exceeded", "max_reconnects", t.maxReconnects)
						t.Close()
					}
				}
				return
			}

			// Process message
			if t.messageHandler != nil {
				t.messageHandler(msg)
			}
		}
	}
}

// readErrors reads from stderr for debugging
func (t *StdioTransport) readErrors() {
	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		// Log stderr output for debugging
		logger.Debug("MCP server stderr", "output", line)
	}
}

// attemptReconnect tries to reconnect the transport using an iterative approach
func (t *StdioTransport) attemptReconnect() {
	t.mu.Lock()
	if t.reconnectActive {
		t.mu.Unlock()
		return // Another reconnection is already in progress
	}
	t.reconnectActive = true
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		t.reconnectActive = false
		t.mu.Unlock()
	}()

	for {
		t.mu.Lock()
		t.reconnectCount++
		reconnectAttempt := t.reconnectCount
		maxReconnects := t.maxReconnects
		parentCtx := t.parentCtx
		t.mu.Unlock()

		if reconnectAttempt > maxReconnects {
			logger.Error("MCP transport max reconnects exceeded", "max_reconnects", maxReconnects)
			return
		}

		logger.Info("MCP transport attempting reconnection", "attempt", reconnectAttempt, "max", maxReconnects)

		// Close current connection
		t.closeConnection()

		// Wait before reconnecting with exponential backoff
		delay := t.reconnectDelay * time.Duration(reconnectAttempt)
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}

		select {
		case <-time.After(delay):
			// Continue with reconnection
		case <-parentCtx.Done():
			logger.Info("MCP transport reconnection canceled due to context")
			return
		}

		// Try to reconnect using parent context
		if err := t.Connect(parentCtx); err != nil {
			logger.Error("MCP transport reconnection failed", "attempt", reconnectAttempt, "error", err)

			t.mu.Lock()
			t.lastError = err
			t.mu.Unlock()

			// Notify error callback
			if t.errorCallback != nil {
				t.errorCallback(err)
			}

			// Continue loop for next attempt
			continue
		}

		// Success
		logger.Info("MCP transport reconnection successful", "attempt", reconnectAttempt)
		t.mu.Lock()
		t.lastError = nil
		t.mu.Unlock()
		return
	}
}

// closeConnection closes the current connection without affecting reconnect state
func (t *StdioTransport) closeConnection() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = false

	if t.cancel != nil {
		t.cancel()
	}

	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.stdout != nil {
		t.stdout.Close()
	}
	if t.stderr != nil {
		t.stderr.Close()
	}

	if t.cmd != nil && t.cmd.Process != nil {
		// Attempt graceful termination first, then force kill
		done := make(chan error, 1)
		go func() {
			done <- t.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited naturally
		case <-time.After(2 * time.Second):
			// Force kill after timeout
			t.cmd.Process.Kill()
			<-done // Wait for actual exit
		}
	}
}
