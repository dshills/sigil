// Package model provides model registry functionality
package model

import (
	"fmt"
	"strings"
	"sync"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
)

// Registry manages model providers and instances
type Registry struct {
	providers map[string]Factory
	models    map[string]Model
	mu        sync.RWMutex
}

// Global registry instance
var defaultRegistry = &Registry{
	providers: make(map[string]Factory),
	models:    make(map[string]Model),
}

// RegisterProvider registers a model provider
func RegisterProvider(name string, provider Factory) error {
	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()

	if _, exists := defaultRegistry.providers[strings.ToLower(name)]; exists {
		return errors.New(errors.ErrorTypeConfig, "RegisterProvider",
			fmt.Sprintf("provider %s already registered", name))
	}

	defaultRegistry.providers[strings.ToLower(name)] = provider
	logger.Info("registered model provider", "provider", name)
	return nil
}

// GetProvider retrieves a registered provider
func GetProvider(name string) (Factory, error) {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()

	provider, exists := defaultRegistry.providers[strings.ToLower(name)]
	if !exists {
		return nil, errors.New(errors.ErrorTypeModel, "GetProvider",
			fmt.Sprintf("provider %s not found", name))
	}

	return provider, nil
}

// CreateModel creates a model instance from configuration
func CreateModel(config ModelConfig) (Model, error) {
	// Check if model already exists
	modelKey := fmt.Sprintf("%s:%s", config.Provider, config.Model)

	defaultRegistry.mu.RLock()
	if model, exists := defaultRegistry.models[modelKey]; exists {
		defaultRegistry.mu.RUnlock()
		return model, nil
	}
	defaultRegistry.mu.RUnlock()

	// Get provider
	provider, err := GetProvider(config.Provider)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeModel, "CreateModel",
			"failed to get provider")
	}

	// Create model
	model, err := provider.CreateModel(config)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeModel, "CreateModel",
			fmt.Sprintf("failed to create model %s", config.Model))
	}

	// Cache the model
	defaultRegistry.mu.Lock()
	defaultRegistry.models[modelKey] = model
	defaultRegistry.mu.Unlock()

	logger.Info("created model instance", "provider", config.Provider, "model", config.Model)
	return model, nil
}

// GetModel retrieves a cached model instance
func GetModel(provider, model string) (Model, error) {
	modelKey := fmt.Sprintf("%s:%s", provider, model)

	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()

	instance, exists := defaultRegistry.models[modelKey]
	if !exists {
		return nil, errors.ModelError("GetModel",
			fmt.Sprintf("model %s not found", modelKey))
	}

	return instance, nil
}

// ListProviders returns all registered providers
func ListProviders() []string {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()

	providers := make([]string, 0, len(defaultRegistry.providers))
	for name := range defaultRegistry.providers {
		providers = append(providers, name)
	}
	return providers
}

// ListModels returns all cached models
func ListModels() []string {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()

	models := make([]string, 0, len(defaultRegistry.models))
	for key := range defaultRegistry.models {
		models = append(models, key)
	}
	return models
}

// ClearModels removes all cached models
func ClearModels() {
	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()

	defaultRegistry.models = make(map[string]Model)
	logger.Debug("cleared model cache")
}

// ParseModelString parses a model string like "openai:gpt-4" or "anthropic:claude-3"
func ParseModelString(modelStr string) (provider, model string, err error) {
	parts := strings.Split(modelStr, ":")
	if len(parts) != 2 {
		return "", "", errors.New(errors.ErrorTypeInput, "ParseModelString",
			"invalid model format, expected provider:model")
	}

	provider = strings.TrimSpace(parts[0])
	model = strings.TrimSpace(parts[1])

	if provider == "" || model == "" {
		return "", "", errors.New(errors.ErrorTypeInput, "ParseModelString",
			"provider and model cannot be empty")
	}

	return provider, model, nil
}
