// Package ollama provides Ollama local model integration for Sigil
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
)

const (
	defaultBaseURL = "http://localhost:11434"
	defaultTimeout = 60 * time.Second // Longer timeout for local inference
)

// Provider implements the Ollama model provider
type Provider struct {
	client *http.Client
}

// NewProvider creates a new Ollama provider
func NewProvider() *Provider {
	return &Provider{
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// CreateModel creates an Ollama model instance
func (p *Provider) CreateModel(config model.ModelConfig) (model.Model, error) {
	baseURL := config.Endpoint
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := defaultTimeout
	if timeoutVal, ok := config.Options["timeout"].(time.Duration); ok {
		timeout = timeoutVal
	}

	return &Model{
		modelName: config.Model,
		baseURL:   baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// ListModels returns available Ollama models
func (p *Provider) ListModels(ctx context.Context) ([]string, error) {
	// We'll try to fetch from Ollama API, but provide defaults if it fails
	defaultModels := []string{
		"llama2",
		"llama2:13b",
		"llama2:70b",
		"codellama",
		"codellama:13b",
		"codellama:34b",
		"mistral",
		"mixtral",
		"neural-chat",
		"starling-lm",
		"phi",
	}

	// Try to get actual models from Ollama
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(defaultBaseURL + "/api/tags")
	if err != nil {
		logger.Debug("failed to fetch Ollama models, using defaults", "error", err)
		return defaultModels, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Debug("Ollama API returned non-200 status, using defaults", "status", resp.StatusCode)
		return defaultModels, nil
	}

	var response struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		logger.Debug("failed to parse Ollama models response, using defaults", "error", err)
		return defaultModels, nil
	}

	models := make([]string, 0, len(response.Models))
	for _, m := range response.Models {
		models = append(models, m.Name)
	}

	if len(models) == 0 {
		return defaultModels, nil
	}

	return models, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "ollama"
}

// Model represents an Ollama model instance
type Model struct {
	modelName string
	baseURL   string
	client    *http.Client
}

// RunPrompt executes a prompt against Ollama API
func (m *Model) RunPrompt(ctx context.Context, input model.PromptInput) (model.PromptOutput, error) {
	start := time.Now()

	// Build request
	req := m.buildRequest(input)

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeModel, "RunPrompt", "failed to marshal request")
	}

	logger.Debug("sending request to Ollama", "model", m.modelName)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", m.baseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeNetwork, "RunPrompt", "failed to create request")
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "Sigil/1.0")

	// Send request
	resp, err := m.client.Do(httpReq)
	if err != nil {
		return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeNetwork, "RunPrompt", "request failed")
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return model.PromptOutput{}, errors.New(errors.ErrorTypeNetwork, "RunPrompt",
			fmt.Sprintf("API error %d: %s", resp.StatusCode, string(respBody)))
	}

	// Ollama returns streaming JSON responses, read until we get the final one
	var finalResp GenerateResponse
	decoder := json.NewDecoder(resp.Body)

	for {
		var response GenerateResponse
		if err := decoder.Decode(&response); err != nil {
			if err == io.EOF {
				break
			}
			return model.PromptOutput{}, errors.Wrap(err, errors.ErrorTypeModel, "RunPrompt", "failed to parse response")
		}

		finalResp.Response += response.Response
		if response.Done {
			finalResp.Done = true
			finalResp.TotalDuration = response.TotalDuration
			finalResp.LoadDuration = response.LoadDuration
			finalResp.PromptEvalCount = response.PromptEvalCount
			finalResp.EvalCount = response.EvalCount
			break
		}
	}

	if !finalResp.Done {
		return model.PromptOutput{}, errors.New(errors.ErrorTypeModel, "RunPrompt", "incomplete response from Ollama")
	}

	// Build output
	output := model.PromptOutput{
		Response:   finalResp.Response,
		TokensUsed: finalResp.PromptEvalCount + finalResp.EvalCount,
		Model:      m.modelName,
		Metadata: map[string]string{
			"total_duration":    fmt.Sprintf("%d", finalResp.TotalDuration),
			"load_duration":     fmt.Sprintf("%d", finalResp.LoadDuration),
			"prompt_eval_count": fmt.Sprintf("%d", finalResp.PromptEvalCount),
			"eval_count":        fmt.Sprintf("%d", finalResp.EvalCount),
		},
	}

	duration := time.Since(start)
	logger.Debug("Ollama request completed", "duration", duration, "tokens", output.TokensUsed)

	return output, nil
}

// GetCapabilities returns the model's capabilities
func (m *Model) GetCapabilities() model.ModelCapabilities {
	// Most Ollama models have large context windows
	maxTokens := 4096 // Conservative default

	// Adjust based on model type
	switch {
	case strings.Contains(m.modelName, "llama2"):
		maxTokens = 4096
	case strings.Contains(m.modelName, "codellama"):
		maxTokens = 16384
	case strings.Contains(m.modelName, "mixtral"):
		maxTokens = 32768
	case strings.Contains(m.modelName, "mistral"):
		maxTokens = 8192
	}

	return model.ModelCapabilities{
		MaxTokens:         maxTokens,
		SupportsImages:    false, // Most Ollama models don't support vision yet
		SupportsTools:     false, // Limited tool support in Ollama
		SupportsStreaming: true,  // Ollama supports streaming
	}
}

// Name returns the model identifier
func (m *Model) Name() string {
	return fmt.Sprintf("ollama:%s", m.modelName)
}

// buildRequest builds the Ollama API request
func (m *Model) buildRequest(input model.PromptInput) GenerateRequest {
	// Build prompt by combining system and user prompts
	var prompt strings.Builder

	if input.SystemPrompt != "" {
		prompt.WriteString("System: ")
		prompt.WriteString(input.SystemPrompt)
		prompt.WriteString("\n\n")
	}

	// Add memory context
	for _, memory := range input.Memory {
		prompt.WriteString(fmt.Sprintf("Assistant: %s\n", memory.Content))
	}

	if input.UserPrompt != "" {
		prompt.WriteString("User: ")
		prompt.WriteString(input.UserPrompt)
		prompt.WriteString("\n")
	}

	// Add file context
	if len(input.Files) > 0 {
		contextText := m.buildFileContext(input.Files)
		if contextText != "" {
			prompt.WriteString(contextText)
		}
	}

	prompt.WriteString("\nAssistant: ")

	req := GenerateRequest{
		Model:  m.modelName,
		Prompt: prompt.String(),
		Stream: false, // We'll handle streaming manually
	}

	// Set options
	if input.Temperature > 0 {
		req.Options = map[string]interface{}{
			"temperature": input.Temperature,
		}
	}

	return req
}

// buildFileContext builds context from file contents
func (m *Model) buildFileContext(files []model.FileContent) string {
	if len(files) == 0 {
		return ""
	}

	var context strings.Builder
	context.WriteString("\nAdditional context files:\n")

	for _, file := range files {
		context.WriteString(fmt.Sprintf("\n--- %s ---\n", file.Path))
		context.WriteString(file.Content)
		context.WriteString("\n")
	}

	return context.String()
}

// Ollama API types

// GenerateRequest represents a generate request
type GenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// GenerateResponse represents a generate response
type GenerateResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	Context            []int  `json:"context,omitempty"`
	TotalDuration      int64  `json:"total_duration,omitempty"`
	LoadDuration       int64  `json:"load_duration,omitempty"`
	PromptEvalCount    int    `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64  `json:"prompt_eval_duration,omitempty"`
	EvalCount          int    `json:"eval_count,omitempty"`
	EvalDuration       int64  `json:"eval_duration,omitempty"`
}
