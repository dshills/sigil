package mcp

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTransport implements Transport interface for testing
type MockTransport struct {
	connected    bool
	messages     chan *RPCMessage
	responses    map[int64]*RPCMessage
	handler      func(*RPCMessage)
	mu           sync.RWMutex
	errorOnSend  bool
	closeChannel chan struct{}
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		messages:     make(chan *RPCMessage, 100),
		responses:    make(map[int64]*RPCMessage),
		closeChannel: make(chan struct{}),
	}
}

func (t *MockTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connected = true
	return nil
}

func (t *MockTransport) Send(msg *RPCMessage) error {
	if t.errorOnSend {
		return assert.AnError
	}

	t.mu.RLock()
	connected := t.connected
	t.mu.RUnlock()

	if !connected {
		return assert.AnError
	}

	// Simulate server response for requests
	if msg.ID != nil {
		go func() {
			time.Sleep(10 * time.Millisecond) // Simulate network delay

			t.mu.RLock()
			response, exists := t.responses[*msg.ID]
			t.mu.RUnlock()

			if exists {
				if t.handler != nil {
					t.handler(response)
				}
			} else {
				// Default response
				defaultResponse := &RPCMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Result:  json.RawMessage(`{"status": "ok"}`),
				}
				if t.handler != nil {
					t.handler(defaultResponse)
				}
			}
		}()
	}

	select {
	case t.messages <- msg:
		return nil
	default:
		return assert.AnError
	}
}

func (t *MockTransport) Receive() (*RPCMessage, error) {
	select {
	case msg := <-t.messages:
		return msg, nil
	case <-t.closeChannel:
		return nil, assert.AnError
	}
}

func (t *MockTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = false
	close(t.closeChannel)
	return nil
}

func (t *MockTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

func (t *MockTransport) SetMessageHandler(handler func(*RPCMessage)) {
	t.handler = handler
}

func (t *MockTransport) SetResponse(id int64, response *RPCMessage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.responses[id] = response
}

func (t *MockTransport) SetErrorOnSend(err bool) {
	t.errorOnSend = err
}

func (t *MockTransport) GetLastMessage() *RPCMessage {
	select {
	case msg := <-t.messages:
		return msg
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func TestProtocolHandler_Initialize(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Set up initialization response
	initResult := InitializeResult{
		ProtocolVersion: "1.0",
		ServerInfo: ServerInfo{
			Name:    "TestServer",
			Version: "1.0.0",
		},
		Capabilities: ServerCapabilities{
			Streaming: false,
			Tools:     true,
			Resources: true,
		},
	}
	resultBytes, _ := json.Marshal(initResult)
	transport.SetResponse(1, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(1),
		Result:  resultBytes,
	})

	// Set up initialized notification response
	transport.SetResponse(2, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(2),
		Result:  json.RawMessage("null"),
	})

	clientInfo := ClientInfo{
		Name:    "TestClient",
		Version: "1.0.0",
	}
	capabilities := ClientCapabilities{
		Streaming: false,
		Tools:     true,
		Resources: true,
	}

	result, err := handler.Initialize(clientInfo, capabilities)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "1.0", result.ProtocolVersion)
	assert.Equal(t, "TestServer", result.ServerInfo.Name)
	assert.True(t, handler.IsInitialized())
}

func TestProtocolHandler_Complete(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Initialize first
	handler.initialized = true
	handler.serverCaps = &ServerCapabilities{Tools: true}

	// Set up completion response
	completionResult := CompletionResult{
		Content: "Test response",
		Model:   "test-model",
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}
	resultBytes, _ := json.Marshal(completionResult)
	transport.SetResponse(1, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(1),
		Result:  resultBytes,
	})

	params := CompletionParams{
		Messages: []Message{
			{Role: "user", Content: "Test message"},
		},
		Model:     "test-model",
		MaxTokens: 100,
	}

	result, err := handler.Complete(params)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Test response", result.Content)
	assert.Equal(t, "test-model", result.Model)
	assert.Equal(t, 30, result.Usage.TotalTokens)
}

func TestProtocolHandler_CallTool(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Initialize first
	handler.initialized = true
	handler.serverCaps = &ServerCapabilities{Tools: true}

	// Set up tool call response
	toolResult := ToolCallResult{
		Content: []ToolCallContent{
			{Type: "text", Text: "Tool executed successfully"},
		},
		IsError: false,
	}
	resultBytes, _ := json.Marshal(toolResult)
	transport.SetResponse(1, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(1),
		Result:  resultBytes,
	})

	args := map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	}

	result, err := handler.CallTool("test_tool", args)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	assert.Equal(t, "text", result.Content[0].Type)
}

func TestProtocolHandler_ListTools(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Initialize first
	handler.initialized = true
	handler.serverCaps = &ServerCapabilities{Tools: true}

	// Set up tools list response
	toolsResponse := struct {
		Tools []ToolDefinition `json:"tools"`
	}{
		Tools: []ToolDefinition{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
	}
	resultBytes, _ := json.Marshal(toolsResponse)
	transport.SetResponse(1, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(1),
		Result:  resultBytes,
	})

	tools, err := handler.ListTools()
	require.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, "test_tool", tools[0].Name)
	assert.Equal(t, "A test tool", tools[0].Description)
}

func TestProtocolHandler_ListResources(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Initialize first
	handler.initialized = true
	handler.serverCaps = &ServerCapabilities{Resources: true}

	// Set up resources list response
	resourcesResponse := struct {
		Resources []ResourceDefinition `json:"resources"`
	}{
		Resources: []ResourceDefinition{
			{
				URI:         "test://resource/1",
				Name:        "Test Resource",
				Description: "A test resource",
				MimeType:    "text/plain",
			},
		},
	}
	resultBytes, _ := json.Marshal(resourcesResponse)
	transport.SetResponse(1, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(1),
		Result:  resultBytes,
	})

	resources, err := handler.ListResources()
	require.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, "test://resource/1", resources[0].URI)
	assert.Equal(t, "Test Resource", resources[0].Name)
}

func TestProtocolHandler_ReadResource(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Initialize first
	handler.initialized = true
	handler.serverCaps = &ServerCapabilities{Resources: true}

	// Set up resource read response
	readResponse := struct {
		Contents []ResourceContent `json:"contents"`
	}{
		Contents: []ResourceContent{
			{
				URI:      "test://resource/1",
				MimeType: "text/plain",
				Text:     "Resource content",
			},
		},
	}
	resultBytes, _ := json.Marshal(readResponse)
	transport.SetResponse(1, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(1),
		Result:  resultBytes,
	})

	content, err := handler.ReadResource("test://resource/1")
	require.NoError(t, err)
	assert.NotNil(t, content)
	assert.Equal(t, "test://resource/1", content.URI)
	assert.Equal(t, "text/plain", content.MimeType)
	assert.Equal(t, "Resource content", content.Text)
}

func TestProtocolHandler_ListPrompts(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Initialize first
	handler.initialized = true

	// Set up prompts list response
	promptsResponse := struct {
		Prompts []PromptTemplate `json:"prompts"`
	}{
		Prompts: []PromptTemplate{
			{
				Name:        "test_prompt",
				Description: "A test prompt",
				Arguments: []TemplateArgument{
					{Name: "subject", Required: true},
				},
			},
		},
	}
	resultBytes, _ := json.Marshal(promptsResponse)
	transport.SetResponse(1, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(1),
		Result:  resultBytes,
	})

	prompts, err := handler.ListPrompts()
	require.NoError(t, err)
	assert.Len(t, prompts, 1)
	assert.Equal(t, "test_prompt", prompts[0].Name)
	assert.Len(t, prompts[0].Arguments, 1)
	assert.True(t, prompts[0].Arguments[0].Required)
}

func TestProtocolHandler_GetPrompt(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Initialize first
	handler.initialized = true

	// Set up prompt get response
	promptResult := PromptResult{
		Description: "Generated prompt",
		Messages: []Message{
			{Role: "user", Content: "Generated prompt content"},
		},
	}
	resultBytes, _ := json.Marshal(promptResult)
	transport.SetResponse(1, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(1),
		Result:  resultBytes,
	})

	args := map[string]interface{}{
		"subject": "testing",
	}

	result, err := handler.GetPrompt("test_prompt", args)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Generated prompt", result.Description)
	assert.Len(t, result.Messages, 1)
	assert.Equal(t, "user", result.Messages[0].Role)
}

func TestProtocolHandler_Ping(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Initialize first
	handler.initialized = true

	// Set up ping response
	pingResult := PingResult{
		Timestamp: time.Now().UnixMilli(),
		Data:      "test-pong",
	}
	resultBytes, _ := json.Marshal(pingResult)
	transport.SetResponse(1, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(1),
		Result:  resultBytes,
	})

	result, err := handler.Ping("test")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-pong", result.Data)
	assert.Greater(t, result.Timestamp, int64(0))
}

func TestProtocolHandler_ErrorHandling(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Test uninitialized protocol
	_, err := handler.Complete(CompletionParams{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test server capabilities check
	handler.initialized = true
	handler.serverCaps = &ServerCapabilities{Tools: false}

	_, err = handler.CallTool("test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support tools")

	// Test transport error
	transport.SetErrorOnSend(true)
	handler.serverCaps = &ServerCapabilities{Tools: true}

	_, err = handler.CallTool("test", map[string]interface{}{})
	assert.Error(t, err)
}

func TestProtocolHandler_ServerNotifications(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Test cancellation notification
	cancelMsg := &RPCMessage{
		JSONRPC: "2.0",
		Method:  "notifications/canceled",
		Params:  json.RawMessage(`{"requestId": 123, "reason": "timeout"}`),
	}

	// This should not panic
	handler.ProcessMessage(cancelMsg)

	// Test progress notification
	progressMsg := &RPCMessage{
		JSONRPC: "2.0",
		Method:  "notifications/progress",
		Params:  json.RawMessage(`{"progressToken": "test", "progress": 50, "total": 100}`),
	}

	handler.ProcessMessage(progressMsg)

	// Test resource update notification
	resourceMsg := &RPCMessage{
		JSONRPC: "2.0",
		Method:  "notifications/resources/updated",
		Params:  json.RawMessage(`{"uri": "test://resource/1"}`),
	}

	handler.ProcessMessage(resourceMsg)
}

func TestProtocolHandler_Shutdown(t *testing.T) {
	transport := NewMockTransport()
	handler := NewProtocolHandler(transport)

	// Initialize first
	handler.initialized = true

	// Set up shutdown response
	transport.SetResponse(1, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(1),
		Result:  json.RawMessage("null"),
	})

	// Set up exit notification response
	transport.SetResponse(2, &RPCMessage{
		JSONRPC: "2.0",
		ID:      int64Ptr(2),
		Result:  json.RawMessage("null"),
	})

	err := handler.Shutdown()
	require.NoError(t, err)
	assert.False(t, handler.IsInitialized())
}

func TestRetryableError(t *testing.T) {
	// Test retryable error
	retryErr := &RetryableError{
		Err:        assert.AnError,
		Retryable:  true,
		RetryAfter: 5,
	}

	assert.True(t, IsRetryableError(retryErr))
	assert.Equal(t, 5, GetRetryDelay(retryErr))
	assert.Equal(t, assert.AnError.Error(), retryErr.Error())

	// Test non-retryable error
	nonRetryErr := &RetryableError{
		Err:       assert.AnError,
		Retryable: false,
	}

	assert.False(t, IsRetryableError(nonRetryErr))

	// Test RPC error
	rpcErr := &RPCError{
		Code:    TransportError,
		Message: "Transport failed",
	}

	assert.True(t, IsRetryableError(rpcErr))

	// Test regular error
	regularErr := assert.AnError
	assert.False(t, IsRetryableError(regularErr))
}

// Helper function for tests
func int64Ptr(i int64) *int64 {
	return &i
}
