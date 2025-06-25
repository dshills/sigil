package model

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockFactory implements Factory for testing
type MockFactory struct {
	mock.Mock
}

func (m *MockFactory) CreateModel(config ModelConfig) (Model, error) {
	args := m.Called(config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(Model), args.Error(1)
}

// MockModel implements Model for testing
type MockModel struct {
	mock.Mock
}

func (m *MockModel) RunPrompt(ctx context.Context, input PromptInput) (PromptOutput, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(PromptOutput), args.Error(1)
}

func (m *MockModel) GetCapabilities() ModelCapabilities {
	args := m.Called()
	return args.Get(0).(ModelCapabilities)
}

func (m *MockModel) Name() string {
	args := m.Called()
	return args.String(0)
}

func TestPromptInput_Structure(t *testing.T) {
	input := PromptInput{
		SystemPrompt: "You are a helpful assistant",
		UserPrompt:   "Hello, world!",
		Files: []FileContent{
			{
				Path:    "main.go",
				Content: "package main",
				Type:    "code",
			},
		},
		Memory: []MemoryEntry{
			{
				Timestamp: "2024-01-01T00:00:00Z",
				Content:   "Previous context",
				Type:      "context",
			},
		},
		MaxTokens:   1000,
		Temperature: 0.7,
		Metadata: map[string]string{
			"request_id": "test-123",
		},
	}

	assert.Equal(t, "You are a helpful assistant", input.SystemPrompt)
	assert.Equal(t, "Hello, world!", input.UserPrompt)
	assert.Len(t, input.Files, 1)
	assert.Equal(t, "main.go", input.Files[0].Path)
	assert.Len(t, input.Memory, 1)
	assert.Equal(t, 1000, input.MaxTokens)
	assert.Equal(t, 0.7, input.Temperature)
	assert.Equal(t, "test-123", input.Metadata["request_id"])
}

func TestPromptOutput_Structure(t *testing.T) {
	output := PromptOutput{
		Response:   "Hello! How can I help you?",
		TokensUsed: 15,
		Model:      "gpt-4",
		Metadata: map[string]string{
			"finish_reason": "stop",
		},
	}

	assert.Equal(t, "Hello! How can I help you?", output.Response)
	assert.Equal(t, 15, output.TokensUsed)
	assert.Equal(t, "gpt-4", output.Model)
	assert.Equal(t, "stop", output.Metadata["finish_reason"])
}

func TestFileContent_Structure(t *testing.T) {
	file := FileContent{
		Path:    "/path/to/file.go",
		Content: "package main\n\nfunc main() {}",
		Type:    "code",
	}

	assert.Equal(t, "/path/to/file.go", file.Path)
	assert.Contains(t, file.Content, "package main")
	assert.Equal(t, "code", file.Type)
}

func TestMemoryEntry_Structure(t *testing.T) {
	entry := MemoryEntry{
		Timestamp: "2024-01-01T12:00:00Z",
		Content:   "User prefers functional programming",
		Type:      "context",
	}

	assert.Equal(t, "2024-01-01T12:00:00Z", entry.Timestamp)
	assert.Equal(t, "User prefers functional programming", entry.Content)
	assert.Equal(t, "context", entry.Type)
}

func TestModelCapabilities_Structure(t *testing.T) {
	caps := ModelCapabilities{
		MaxTokens:         4096,
		SupportsImages:    true,
		SupportsTools:     true,
		SupportsStreaming: false,
	}

	assert.Equal(t, 4096, caps.MaxTokens)
	assert.True(t, caps.SupportsImages)
	assert.True(t, caps.SupportsTools)
	assert.False(t, caps.SupportsStreaming)
}

func TestModelConfig_Structure(t *testing.T) {
	config := ModelConfig{
		Provider: "openai",
		Model:    "gpt-4",
		APIKey:   "sk-test-key",
		Endpoint: "https://api.openai.com/v1",
		Options: map[string]interface{}{
			"temperature": 0.7,
			"max_tokens":  1000,
		},
	}

	assert.Equal(t, "openai", config.Provider)
	assert.Equal(t, "gpt-4", config.Model)
	assert.Equal(t, "sk-test-key", config.APIKey)
	assert.Equal(t, "https://api.openai.com/v1", config.Endpoint)
	assert.Equal(t, 0.7, config.Options["temperature"])
	assert.Equal(t, 1000, config.Options["max_tokens"])
}

func TestMockModel_Implementation(t *testing.T) {
	mockModel := &MockModel{}

	// Set up expectations
	mockModel.On("Name").Return("test-model")
	mockModel.On("GetCapabilities").Return(ModelCapabilities{
		MaxTokens:         1000,
		SupportsImages:    false,
		SupportsTools:     false,
		SupportsStreaming: false,
	})

	ctx := context.Background()
	input := PromptInput{
		SystemPrompt: "Test",
		UserPrompt:   "Hello",
	}
	expectedOutput := PromptOutput{
		Response:   "Hi there!",
		TokensUsed: 5,
		Model:      "test-model",
	}
	mockModel.On("RunPrompt", ctx, input).Return(expectedOutput, nil)

	// Test the mock
	assert.Equal(t, "test-model", mockModel.Name())

	caps := mockModel.GetCapabilities()
	assert.Equal(t, 1000, caps.MaxTokens)
	assert.False(t, caps.SupportsImages)

	output, err := mockModel.RunPrompt(ctx, input)
	assert.NoError(t, err)
	assert.Equal(t, "Hi there!", output.Response)
	assert.Equal(t, 5, output.TokensUsed)

	mockModel.AssertExpectations(t)
}

func TestMockFactory_Implementation(t *testing.T) {
	mockFactory := &MockFactory{}
	mockModel := &MockModel{}

	config := ModelConfig{
		Provider: "test",
		Model:    "test-model",
		APIKey:   "test-key",
	}

	// Set up expectations
	mockFactory.On("CreateModel", config).Return(mockModel, nil)

	// Test the mock
	model, err := mockFactory.CreateModel(config)
	assert.NoError(t, err)
	assert.Equal(t, mockModel, model)

	mockFactory.AssertExpectations(t)
}

func TestModelConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config ModelConfig
		valid  bool
	}{
		{
			name: "valid openai config",
			config: ModelConfig{
				Provider: "openai",
				Model:    "gpt-4",
				APIKey:   "sk-test",
			},
			valid: true,
		},
		{
			name: "valid anthropic config",
			config: ModelConfig{
				Provider: "anthropic",
				Model:    "claude-3",
				APIKey:   "sk-ant-test",
			},
			valid: true,
		},
		{
			name: "valid ollama config",
			config: ModelConfig{
				Provider: "ollama",
				Model:    "llama2",
				Endpoint: "http://localhost:11434",
			},
			valid: true,
		},
		{
			name: "empty provider",
			config: ModelConfig{
				Model:  "gpt-4",
				APIKey: "sk-test",
			},
			valid: false,
		},
		{
			name: "empty model",
			config: ModelConfig{
				Provider: "openai",
				APIKey:   "sk-test",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation - config should have required fields
			if tt.valid {
				assert.NotEmpty(t, tt.config.Provider)
				assert.NotEmpty(t, tt.config.Model)
			} else {
				// At least one required field should be empty
				isEmpty := tt.config.Provider == "" || tt.config.Model == ""
				assert.True(t, isEmpty)
			}
		})
	}
}

func TestPromptInput_WithMultipleFiles(t *testing.T) {
	input := PromptInput{
		SystemPrompt: "Code reviewer",
		UserPrompt:   "Review these files",
		Files: []FileContent{
			{Path: "main.go", Content: "package main", Type: "code"},
			{Path: "utils.go", Content: "package main", Type: "code"},
			{Path: "README.md", Content: "# Project", Type: "text"},
		},
	}

	assert.Len(t, input.Files, 3)

	codeFiles := 0
	textFiles := 0
	for _, file := range input.Files {
		switch file.Type {
		case "code":
			codeFiles++
		case "text":
			textFiles++
		}
	}

	assert.Equal(t, 2, codeFiles)
	assert.Equal(t, 1, textFiles)
}

func TestPromptInput_WithMultipleMemoryEntries(t *testing.T) {
	input := PromptInput{
		SystemPrompt: "Assistant",
		UserPrompt:   "Continue our conversation",
		Memory: []MemoryEntry{
			{Type: "context", Content: "User likes Go", Timestamp: "2024-01-01"},
			{Type: "decision", Content: "Use testify for testing", Timestamp: "2024-01-02"},
			{Type: "summary", Content: "Working on test coverage", Timestamp: "2024-01-03"},
		},
	}

	assert.Len(t, input.Memory, 3)

	typeCount := make(map[string]int)
	for _, entry := range input.Memory {
		typeCount[entry.Type]++
	}

	assert.Equal(t, 1, typeCount["context"])
	assert.Equal(t, 1, typeCount["decision"])
	assert.Equal(t, 1, typeCount["summary"])
}

func TestModelCapabilities_FeatureChecks(t *testing.T) {
	// High-capability model
	advancedModel := ModelCapabilities{
		MaxTokens:         32000,
		SupportsImages:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	}

	// Basic model
	basicModel := ModelCapabilities{
		MaxTokens:         4000,
		SupportsImages:    false,
		SupportsTools:     false,
		SupportsStreaming: false,
	}

	// Test advanced model capabilities
	assert.True(t, advancedModel.MaxTokens > 16000)
	assert.True(t, advancedModel.SupportsImages)
	assert.True(t, advancedModel.SupportsTools)
	assert.True(t, advancedModel.SupportsStreaming)

	// Test basic model limitations
	assert.True(t, basicModel.MaxTokens < 8000)
	assert.False(t, basicModel.SupportsImages)
	assert.False(t, basicModel.SupportsTools)
	assert.False(t, basicModel.SupportsStreaming)
}

func TestProviderNaming_Conventions(t *testing.T) {
	providers := []string{"openai", "anthropic", "ollama", "mcp"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			// Provider names should be lowercase
			assert.Equal(t, strings.ToLower(provider), provider)

			// Provider names should not contain spaces
			assert.NotContains(t, provider, " ")

			// Provider names should not be empty
			assert.NotEmpty(t, provider)
		})
	}
}

func TestMemoryEntry_Types(t *testing.T) {
	validTypes := []string{"context", "decision", "summary"}

	for _, entryType := range validTypes {
		t.Run(entryType, func(t *testing.T) {
			entry := MemoryEntry{
				Type:      entryType,
				Content:   "Test content",
				Timestamp: "2024-01-01",
			}

			assert.Equal(t, entryType, entry.Type)
			assert.NotEmpty(t, entry.Content)
			assert.NotEmpty(t, entry.Timestamp)
		})
	}
}

func TestFileContent_Types(t *testing.T) {
	validTypes := []string{"text", "code", "markdown", "json", "yaml"}

	for _, fileType := range validTypes {
		t.Run(fileType, func(t *testing.T) {
			file := FileContent{
				Path:    "test." + fileType,
				Content: "test content",
				Type:    fileType,
			}

			assert.Equal(t, fileType, file.Type)
			assert.Contains(t, file.Path, fileType)
			assert.NotEmpty(t, file.Content)
		})
	}
}
