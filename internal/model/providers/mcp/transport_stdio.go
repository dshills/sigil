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

	messageHandler func(*RPCMessage)
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(command string, args []string, env []string, config TransportConfig) *StdioTransport {
	return &StdioTransport{
		command: command,
		args:    args,
		env:     env,
		config:  config,
	}
}

// SetMessageHandler sets the handler for incoming messages
func (t *StdioTransport) SetMessageHandler(handler func(*RPCMessage)) {
	t.messageHandler = handler
}

// Connect starts the MCP server process
func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

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
	done := make(chan error, 1)
	go func() {
		done <- t.cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited normally
	case <-time.After(5 * time.Second):
		// Force kill after timeout
		t.cmd.Process.Kill()
		<-done
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
					// TODO: Handle read error (reconnect logic)
					t.Close()
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
		// TODO: Log stderr output for debugging
		_ = scanner.Text()
	}
}
