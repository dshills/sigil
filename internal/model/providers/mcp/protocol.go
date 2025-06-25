package mcp

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
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

// Error implements the error interface
func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
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
	// Handle server-initiated requests and notifications
	if msg.Method != "" {
		// This is a request or notification from the server
		h.handleServerMessage(msg)
	}
}

// IsInitialized returns whether the protocol has been initialized
func (h *ProtocolHandler) IsInitialized() bool {
	return h.initialized
}

// GetServerCapabilities returns the server's capabilities
func (h *ProtocolHandler) GetServerCapabilities() *ServerCapabilities {
	return h.serverCaps
}

// Tool calling support

// ToolCallParams represents parameters for calling a tool
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCallResult represents the result of a tool call
type ToolCallResult struct {
	Content []ToolCallContent `json:"content"`
	IsError bool              `json:"isError,omitempty"`
}

// ToolCallContent represents content returned by a tool
type ToolCallContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
}

// CallTool calls a tool on the server
func (h *ProtocolHandler) CallTool(name string, arguments map[string]interface{}) (*ToolCallResult, error) {
	if !h.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if h.serverCaps == nil || !h.serverCaps.Tools {
		return nil, fmt.Errorf("server does not support tools")
	}

	params := ToolCallParams{
		Name:      name,
		Arguments: arguments,
	}

	result, err := h.Request("tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	var toolResult ToolCallResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	return &toolResult, nil
}

// ListTools lists available tools on the server
func (h *ProtocolHandler) ListTools() ([]ToolDefinition, error) {
	if !h.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if h.serverCaps == nil || !h.serverCaps.Tools {
		return nil, fmt.Errorf("server does not support tools")
	}

	result, err := h.Request("tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	var response struct {
		Tools []ToolDefinition `json:"tools"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse tools list: %w", err)
	}

	return response.Tools, nil
}

// Resource management support

// ResourceParams represents parameters for resource operations
type ResourceParams struct {
	URI string `json:"uri"`
}

// ResourceContent represents the content of a resource
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     []byte `json:"blob,omitempty"`
}

// ListResources lists available resources on the server
func (h *ProtocolHandler) ListResources() ([]ResourceDefinition, error) {
	if !h.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if h.serverCaps == nil || !h.serverCaps.Resources {
		return nil, fmt.Errorf("server does not support resources")
	}

	result, err := h.Request("resources/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	var response struct {
		Resources []ResourceDefinition `json:"resources"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse resources list: %w", err)
	}

	return response.Resources, nil
}

// ReadResource reads the content of a resource
func (h *ProtocolHandler) ReadResource(uri string) (*ResourceContent, error) {
	if !h.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	if h.serverCaps == nil || !h.serverCaps.Resources {
		return nil, fmt.Errorf("server does not support resources")
	}

	params := ResourceParams{URI: uri}
	result, err := h.Request("resources/read", params)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource: %w", err)
	}

	var response struct {
		Contents []ResourceContent `json:"contents"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse resource content: %w", err)
	}

	if len(response.Contents) == 0 {
		return nil, fmt.Errorf("no content returned for resource: %s", uri)
	}

	return &response.Contents[0], nil
}

// SubscribeToResource subscribes to changes in a resource
func (h *ProtocolHandler) SubscribeToResource(uri string) error {
	if !h.initialized {
		return fmt.Errorf("protocol not initialized")
	}

	if h.serverCaps == nil || !h.serverCaps.Resources {
		return fmt.Errorf("server does not support resources")
	}

	params := ResourceParams{URI: uri}
	_, err := h.Request("resources/subscribe", params)
	if err != nil {
		return fmt.Errorf("failed to subscribe to resource: %w", err)
	}

	return nil
}

// UnsubscribeFromResource unsubscribes from changes in a resource
func (h *ProtocolHandler) UnsubscribeFromResource(uri string) error {
	if !h.initialized {
		return fmt.Errorf("protocol not initialized")
	}

	if h.serverCaps == nil || !h.serverCaps.Resources {
		return fmt.Errorf("server does not support resources")
	}

	params := ResourceParams{URI: uri}
	_, err := h.Request("resources/unsubscribe", params)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from resource: %w", err)
	}

	return nil
}

// Prompt template support

// PromptTemplate represents a prompt template
type PromptTemplate struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Arguments   []TemplateArgument     `json:"arguments,omitempty"`
}

// TemplateArgument represents an argument for a prompt template
type TemplateArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptParams represents parameters for getting a prompt
type PromptParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// PromptResult represents the result of getting a prompt
type PromptResult struct {
	Description string    `json:"description,omitempty"`
	Messages    []Message `json:"messages"`
}

// ListPrompts lists available prompt templates
func (h *ProtocolHandler) ListPrompts() ([]PromptTemplate, error) {
	if !h.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	result, err := h.Request("prompts/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}

	var response struct {
		Prompts []PromptTemplate `json:"prompts"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse prompts list: %w", err)
	}

	return response.Prompts, nil
}

// GetPrompt gets a prompt template with arguments
func (h *ProtocolHandler) GetPrompt(name string, arguments map[string]interface{}) (*PromptResult, error) {
	if !h.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	params := PromptParams{
		Name:      name,
		Arguments: arguments,
	}

	result, err := h.Request("prompts/get", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt: %w", err)
	}

	var promptResult PromptResult
	if err := json.Unmarshal(result, &promptResult); err != nil {
		return nil, fmt.Errorf("failed to parse prompt result: %w", err)
	}

	return &promptResult, nil
}

// Logging support

// LogLevel represents the level of a log message
type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
)

// LogParams represents parameters for logging
type LogParams struct {
	Level  LogLevel `json:"level"`
	Data   string   `json:"data"`
	Logger string   `json:"logger,omitempty"`
}

// SendLog sends a log message to the server
func (h *ProtocolHandler) SendLog(level LogLevel, data string, logger string) error {
	params := LogParams{
		Level:  level,
		Data:   data,
		Logger: logger,
	}

	return h.Notify("notifications/message", params)
}

// Server message handling

// handleServerMessage handles incoming requests and notifications from the server
func (h *ProtocolHandler) handleServerMessage(msg *RPCMessage) {
	// Handle server-initiated notifications
	switch msg.Method {
	case "notifications/initialized":
		// Server acknowledges initialization
	case "notifications/cancelled":
		// Handle request cancellation
		h.handleCancellation(msg)
	case "notifications/progress":
		// Handle progress updates
		h.handleProgress(msg)
	case "notifications/resources/updated":
		// Handle resource update notifications
		h.handleResourceUpdate(msg)
	case "notifications/resources/list_changed":
		// Handle resource list changes
		h.handleResourceListChange(msg)
	default:
		// Unknown notification - log it
		h.SendLog(LogLevelWarning, fmt.Sprintf("Unknown server notification: %s", msg.Method), "mcp-client")
	}
}

// handleCancellation handles request cancellation notifications
func (h *ProtocolHandler) handleCancellation(msg *RPCMessage) {
	var params struct {
		RequestID int64  `json:"requestId"`
		Reason    string `json:"reason,omitempty"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return
	}

	// Cancel the pending request
	h.mu.Lock()
	if responseChan, ok := h.pendingRequests[params.RequestID]; ok {
		delete(h.pendingRequests, params.RequestID)
		close(responseChan)
	}
	h.mu.Unlock()
}

// handleProgress handles progress update notifications
func (h *ProtocolHandler) handleProgress(msg *RPCMessage) {
	var params struct {
		ProgressToken string  `json:"progressToken"`
		Progress      float64 `json:"progress"`
		Total         float64 `json:"total,omitempty"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return
	}

	// Log progress update
	h.SendLog(LogLevelInfo, fmt.Sprintf("Progress: %.2f%%", (params.Progress/params.Total)*100), "mcp-client")
}

// handleResourceUpdate handles resource update notifications
func (h *ProtocolHandler) handleResourceUpdate(msg *RPCMessage) {
	var params struct {
		URI string `json:"uri"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return
	}

	// Log resource update
	h.SendLog(LogLevelInfo, fmt.Sprintf("Resource updated: %s", params.URI), "mcp-client")
}

// handleResourceListChange handles resource list change notifications
func (h *ProtocolHandler) handleResourceListChange(msg *RPCMessage) {
	// Log resource list change
	h.SendLog(LogLevelInfo, "Resource list changed", "mcp-client")
}

// Error handling and recovery

// RetryableError represents an error that can be retried
type RetryableError struct {
	Err        error
	Retryable  bool
	RetryAfter int // seconds
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if rErr, ok := err.(*RetryableError); ok {
		return rErr.Retryable
	}

	// Check RPC error codes for retryable conditions
	if rpcErr, ok := err.(*RPCError); ok {
		switch rpcErr.Code {
		case TransportError, ServerError:
			return true
		case InternalError:
			return true // Server internal errors might be transient
		default:
			return false
		}
	}

	return false
}

// GetRetryDelay returns the recommended retry delay for an error
func GetRetryDelay(err error) int {
	if rErr, ok := err.(*RetryableError); ok && rErr.RetryAfter > 0 {
		return rErr.RetryAfter
	}
	return 1 // Default 1 second
}

// Health checking

// PingParams represents parameters for ping
type PingParams struct {
	Timestamp int64  `json:"timestamp"`
	Data      string `json:"data,omitempty"`
}

// PingResult represents the result of a ping
type PingResult struct {
	Timestamp int64  `json:"timestamp"`
	Data      string `json:"data,omitempty"`
}

// Ping sends a ping to check server health
func (h *ProtocolHandler) Ping(data string) (*PingResult, error) {
	if !h.initialized {
		return nil, fmt.Errorf("protocol not initialized")
	}

	params := PingParams{
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
	}

	result, err := h.Request("ping", params)
	if err != nil {
		return nil, fmt.Errorf("ping failed: %w", err)
	}

	var pingResult PingResult
	if err := json.Unmarshal(result, &pingResult); err != nil {
		return nil, fmt.Errorf("failed to parse ping result: %w", err)
	}

	return &pingResult, nil
}
