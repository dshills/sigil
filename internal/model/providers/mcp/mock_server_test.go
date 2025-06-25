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

// MockServer implements a simple MCP server for testing
type MockServer struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	messages   chan *RPCMessage
	responses  map[string]*RPCMessage
	tools      []ToolDefinition
	resources  []ResourceDefinition
	prompts    []PromptTemplate
	mu         sync.Mutex
	running    bool
	serverInfo ServerInfo
}

// NewMockServer creates a new mock MCP server
func NewMockServer() *MockServer {
	return &MockServer{
		messages:  make(chan *RPCMessage, 100),
		responses: make(map[string]*RPCMessage),
		tools: []ToolDefinition{
			{
				Name:        "test_tool",
				Description: "A test tool for verification",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]interface{}{
							"type":        "string",
							"description": "Message to echo",
						},
					},
					"required": []string{"message"},
				},
			},
		},
		resources: []ResourceDefinition{
			{
				URI:         "test://resource/1",
				Name:        "Test Resource",
				Description: "A test resource",
				MimeType:    "text/plain",
			},
		},
		prompts: []PromptTemplate{
			{
				Name:        "test_prompt",
				Description: "A test prompt template",
				Arguments: []TemplateArgument{
					{
						Name:        "subject",
						Description: "The subject to write about",
						Required:    true,
					},
				},
			},
		},
		serverInfo: ServerInfo{
			Name:    "MockMCPServer",
			Version: "1.0.0-test",
		},
	}
}

// Start starts the mock server
func (m *MockServer) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("server already running")
	}

	// Create a mock command that will run our mock server
	m.cmd = exec.CommandContext(ctx, "cat")
	
	stdin, err := m.cmd.StdinPipe()
	if err != nil {
		return err
	}
	m.stdin = stdin

	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	m.stdout = stdout

	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return err
	}
	m.stderr = stderr

	if err := m.cmd.Start(); err != nil {
		return err
	}

	m.running = true

	// Start message handler
	go m.handleMessages()

	return nil
}

// Stop stops the mock server
func (m *MockServer) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	if m.stdin != nil {
		m.stdin.Close()
	}
	if m.stdout != nil {
		m.stdout.Close()
	}
	if m.stderr != nil {
		m.stderr.Close()
	}

	if m.cmd != nil {
		m.cmd.Process.Kill()
		m.cmd.Wait()
	}

	m.running = false
	return nil
}

// handleMessages handles incoming JSON-RPC messages
func (m *MockServer) handleMessages() {
	scanner := bufio.NewScanner(m.stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var msg RPCMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		// Handle the message
		response := m.processMessage(&msg)
		if response != nil {
			// Send response
			responseBytes, _ := json.Marshal(response)
			fmt.Fprintf(m.stdin, "%s\n", responseBytes)
		}
	}
}

// processMessage processes an incoming message and returns a response
func (m *MockServer) processMessage(msg *RPCMessage) *RPCMessage {
	if msg.Method == "" {
		// This is a response, not a request
		return nil
	}

	switch msg.Method {
	case "initialize":
		return m.handleInitialize(msg)
	case "initialized":
		return nil // Notification, no response
	case "shutdown":
		return m.handleShutdown(msg)
	case "exit":
		return nil // Notification, no response
	case "completion/complete":
		return m.handleCompletion(msg)
	case "tools/list":
		return m.handleToolsList(msg)
	case "tools/call":
		return m.handleToolCall(msg)
	case "resources/list":
		return m.handleResourcesList(msg)
	case "resources/read":
		return m.handleResourceRead(msg)
	case "prompts/list":
		return m.handlePromptsList(msg)
	case "prompts/get":
		return m.handlePromptGet(msg)
	case "ping":
		return m.handlePing(msg)
	default:
		return &RPCMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: &RPCError{
				Code:    MethodNotFound,
				Message: fmt.Sprintf("Method not found: %s", msg.Method),
			},
		}
	}
}

// handleInitialize handles the initialize request
func (m *MockServer) handleInitialize(msg *RPCMessage) *RPCMessage {
	result := InitializeResult{
		ProtocolVersion: "1.0",
		ServerInfo:      m.serverInfo,
		Capabilities: ServerCapabilities{
			Streaming: false,
			Tools:     true,
			Resources: true,
		},
		AvailableTools:     m.tools,
		AvailableResources: m.resources,
	}

	resultBytes, _ := json.Marshal(result)
	return &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  resultBytes,
	}
}

// handleShutdown handles the shutdown request
func (m *MockServer) handleShutdown(msg *RPCMessage) *RPCMessage {
	return &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  json.RawMessage("null"),
	}
}

// handleCompletion handles completion requests
func (m *MockServer) handleCompletion(msg *RPCMessage) *RPCMessage {
	var params CompletionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return &RPCMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Invalid completion parameters",
			},
		}
	}

	// Create mock completion response
	result := CompletionResult{
		Content: fmt.Sprintf("Mock response to: %s", params.Messages[len(params.Messages)-1].Content),
		Model:   params.Model,
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	resultBytes, _ := json.Marshal(result)
	return &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  resultBytes,
	}
}

// handleToolsList handles tools/list requests
func (m *MockServer) handleToolsList(msg *RPCMessage) *RPCMessage {
	result := struct {
		Tools []ToolDefinition `json:"tools"`
	}{
		Tools: m.tools,
	}

	resultBytes, _ := json.Marshal(result)
	return &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  resultBytes,
	}
}

// handleToolCall handles tools/call requests
func (m *MockServer) handleToolCall(msg *RPCMessage) *RPCMessage {
	var params ToolCallParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return &RPCMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Invalid tool call parameters",
			},
		}
	}

	// Mock tool execution
	result := ToolCallResult{
		Content: []ToolCallContent{
			{
				Type: "text",
				Text: fmt.Sprintf("Tool %s executed with args: %v", params.Name, params.Arguments),
			},
		},
		IsError: false,
	}

	resultBytes, _ := json.Marshal(result)
	return &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  resultBytes,
	}
}

// handleResourcesList handles resources/list requests
func (m *MockServer) handleResourcesList(msg *RPCMessage) *RPCMessage {
	result := struct {
		Resources []ResourceDefinition `json:"resources"`
	}{
		Resources: m.resources,
	}

	resultBytes, _ := json.Marshal(result)
	return &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  resultBytes,
	}
}

// handleResourceRead handles resources/read requests
func (m *MockServer) handleResourceRead(msg *RPCMessage) *RPCMessage {
	var params ResourceParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return &RPCMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Invalid resource parameters",
			},
		}
	}

	result := struct {
		Contents []ResourceContent `json:"contents"`
	}{
		Contents: []ResourceContent{
			{
				URI:      params.URI,
				MimeType: "text/plain",
				Text:     "Mock resource content for " + params.URI,
			},
		},
	}

	resultBytes, _ := json.Marshal(result)
	return &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  resultBytes,
	}
}

// handlePromptsList handles prompts/list requests
func (m *MockServer) handlePromptsList(msg *RPCMessage) *RPCMessage {
	result := struct {
		Prompts []PromptTemplate `json:"prompts"`
	}{
		Prompts: m.prompts,
	}

	resultBytes, _ := json.Marshal(result)
	return &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  resultBytes,
	}
}

// handlePromptGet handles prompts/get requests
func (m *MockServer) handlePromptGet(msg *RPCMessage) *RPCMessage {
	var params PromptParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return &RPCMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Invalid prompt parameters",
			},
		}
	}

	result := PromptResult{
		Description: "Mock prompt result",
		Messages: []Message{
			{
				Role:    "user",
				Content: fmt.Sprintf("Mock prompt %s with args: %v", params.Name, params.Arguments),
			},
		},
	}

	resultBytes, _ := json.Marshal(result)
	return &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  resultBytes,
	}
}

// handlePing handles ping requests
func (m *MockServer) handlePing(msg *RPCMessage) *RPCMessage {
	var params PingParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return &RPCMessage{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Invalid ping parameters",
			},
		}
	}

	result := PingResult{
		Timestamp: time.Now().UnixMilli(),
		Data:      params.Data + "-pong",
	}

	resultBytes, _ := json.Marshal(result)
	return &RPCMessage{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  resultBytes,
	}
}