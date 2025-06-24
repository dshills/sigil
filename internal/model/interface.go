package model

import (
	"context"
)

// PromptInput represents the input to a model prompt.
type PromptInput struct {
	SystemPrompt string            // System-level instructions
	UserPrompt   string            // User's request or question
	Files        []FileContent     // Files to include in context
	Memory       []MemoryEntry     // Memory entries to include
	MaxTokens    int               // Maximum tokens for response
	Temperature  float64           // Temperature for response generation
	Metadata     map[string]string // Additional metadata
}

// FileContent represents a file's content to be included in a prompt.
type FileContent struct {
	Path    string
	Content string
	Type    string // "text", "code", etc.
}

// MemoryEntry represents a memory entry from the Markdown-based memory system.
type MemoryEntry struct {
	Timestamp string
	Content   string
	Type      string // "context", "decision", "summary"
}

// PromptOutput represents the output from a model.
type PromptOutput struct {
	Response   string            // The model's response
	TokensUsed int               // Number of tokens used
	Model      string            // Model identifier used
	Metadata   map[string]string // Additional metadata
}

// Model is the interface that all LLM backends must implement.
type Model interface {
	// RunPrompt executes a prompt and returns the response
	RunPrompt(ctx context.Context, input PromptInput) (PromptOutput, error)

	// GetCapabilities returns the model's capabilities
	GetCapabilities() ModelCapabilities

	// Name returns the model provider name
	Name() string
}

// ModelCapabilities describes what a model can do.
type ModelCapabilities struct {
	MaxTokens         int
	SupportsImages    bool
	SupportsTools     bool
	SupportsStreaming bool
}

// ModelConfig holds configuration for a model.
type ModelConfig struct {
	Provider string                 // "openai", "anthropic", "ollama", "mcp"
	Model    string                 // Specific model name
	APIKey   string                 // API key if required
	Endpoint string                 // Custom endpoint if applicable
	Options  map[string]interface{} // Provider-specific options
}

// Factory creates a Model instance based on configuration
type Factory interface {
	CreateModel(config ModelConfig) (Model, error)
}
