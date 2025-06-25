package mcp

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// RPCMessage represents a JSON-RPC 2.0 message
type RPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// MCP-specific error codes
const (
	ServerError         = -32000
	TransportError      = -32001
	InitializationError = -32002
	ToolExecutionError  = -32003
	ResourceError       = -32004
)

// InitializeParams represents parameters for the initialize method
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
	Capabilities    ClientCapabilities     `json:"capabilities"`
	Configuration   map[string]interface{} `json:"configuration,omitempty"`
}

// ClientInfo contains information about the client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities defines what the client supports
type ClientCapabilities struct {
	Streaming    bool     `json:"streaming"`
	Tools        bool     `json:"tools"`
	Resources    bool     `json:"resources"`
	Experimental []string `json:"experimental,omitempty"`
}

// InitializeResult represents the server's response to initialization
type InitializeResult struct {
	ProtocolVersion    string               `json:"protocolVersion"`
	ServerInfo         ServerInfo           `json:"serverInfo"`
	Capabilities       ServerCapabilities   `json:"capabilities"`
	AvailableTools     []ToolDefinition     `json:"availableTools,omitempty"`
	AvailableResources []ResourceDefinition `json:"availableResources,omitempty"`
}

// ServerInfo contains information about the server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities defines what the server supports
type ServerCapabilities struct {
	Streaming    bool     `json:"streaming"`
	Tools        bool     `json:"tools"`
	Resources    bool     `json:"resources"`
	Experimental []string `json:"experimental,omitempty"`
}

// ToolDefinition describes a tool available on the server
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ResourceDefinition describes a resource available on the server
type ResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// CompletionParams represents parameters for text completion
type CompletionParams struct {
	Messages    []Message              `json:"messages"`
	Model       string                 `json:"model,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	MaxTokens   int                    `json:"maxTokens,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Message represents a conversation message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResult represents the completion response
type CompletionResult struct {
	Content  string                 `json:"content"`
	Model    string                 `json:"model,omitempty"`
	Usage    *Usage                 `json:"usage,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Usage tracks token usage
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// ProtocolHandler manages MCP protocol communication
type ProtocolHandler struct {
	transport       Transport
	requestID       atomic.Int64
	pendingRequests map[int64]chan *RPCMessage
	mu              sync.RWMutex
	initialized     bool
	serverCaps      *ServerCapabilities
}

// NewProtocolHandler creates a new protocol handler
func NewProtocolHandler(transport Transport) *ProtocolHandler {
	return &ProtocolHandler{
		transport:       transport,
		pendingRequests: make(map[int64]chan *RPCMessage),
	}
}

// Initialize performs the MCP initialization handshake
func (h *ProtocolHandler) Initialize(clientInfo ClientInfo, capabilities ClientCapabilities) (*InitializeResult, error) {
	params := InitializeParams{
		ProtocolVersion: "1.0",
		ClientInfo:      clientInfo,
		Capabilities:    capabilities,
	}

	result, err := h.Request("initialize", params)
	if err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	var initResult InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return nil, fmt.Errorf("failed to parse initialization result: %w", err)
	}

	// Send initialized notification
	if err := h.Notify("initialized", nil); err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	h.initialized = true
	h.serverCaps = &initResult.Capabilities
	return &initResult, nil
}

// Complete performs text completion
func (h *ProtocolHandler) Complete(params CompletionParams) (*CompletionResult, error) {
	if !h.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	result, err := h.Request("completion/complete", params)
	if err != nil {
		return nil, err
	}

	var completionResult CompletionResult
	if err := json.Unmarshal(result, &completionResult); err != nil {
		return nil, fmt.Errorf("failed to parse completion result: %w", err)
	}

	return &completionResult, nil
}

// Shutdown gracefully shuts down the connection
func (h *ProtocolHandler) Shutdown() error {
	if !h.initialized {
		return nil
	}

	_, err := h.Request("shutdown", nil)
	if err != nil {
		return fmt.Errorf("shutdown failed: %w", err)
	}

	// Send exit notification
	if err := h.Notify("exit", nil); err != nil {
		return fmt.Errorf("failed to send exit notification: %w", err)
	}

	h.initialized = false
	return h.transport.Close()
}

// Request sends a request and waits for a response
func (h *ProtocolHandler) Request(method string, params interface{}) (json.RawMessage, error) {
	id := h.requestID.Add(1)

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	msg := &RPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  paramsJSON,
	}

	// Create response channel
	responseChan := make(chan *RPCMessage, 1)
	h.mu.Lock()
	h.pendingRequests[id] = responseChan
	h.mu.Unlock()

	// Send request
	if err := h.transport.Send(msg); err != nil {
		h.mu.Lock()
		delete(h.pendingRequests, id)
		h.mu.Unlock()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	response := <-responseChan

	h.mu.Lock()
	delete(h.pendingRequests, id)
	h.mu.Unlock()

	if response.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", response.Error.Code, response.Error.Message)
	}

	return response.Result, nil
}

// Notify sends a notification (no response expected)
func (h *ProtocolHandler) Notify(method string, params interface{}) error {
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	msg := &RPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}

	return h.transport.Send(msg)
}

// ProcessMessage handles incoming messages from the transport
func (h *ProtocolHandler) ProcessMessage(msg *RPCMessage) {
	if msg == nil {
		return
	}

	if msg.ID != nil {
		// This is a response to a request
		h.mu.RLock()
		responseChan, ok := h.pendingRequests[*msg.ID]
		h.mu.RUnlock()

		if ok {
			responseChan <- msg
		}
	}
	// TODO: Handle server-initiated requests and notifications
}

// IsInitialized returns whether the protocol has been initialized
func (h *ProtocolHandler) IsInitialized() bool {
	return h.initialized
}

// GetServerCapabilities returns the server's capabilities
func (h *ProtocolHandler) GetServerCapabilities() *ServerCapabilities {
	return h.serverCaps
}
