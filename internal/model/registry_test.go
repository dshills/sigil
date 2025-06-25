package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistry_Structure(t *testing.T) {
	registry := &Registry{
		providers: make(map[string]Factory),
		models:    make(map[string]Model),
	}

	assert.NotNil(t, registry.providers)
	assert.NotNil(t, registry.models)
	assert.Len(t, registry.providers, 0)
	assert.Len(t, registry.models, 0)
}

func TestRegisterProvider(t *testing.T) {
	// Clean up the default registry for testing
	originalProviders := defaultRegistry.providers
	defer func() {
		defaultRegistry.providers = originalProviders
	}()
	defaultRegistry.providers = make(map[string]Factory)

	mockFactory := &MockFactory{}

	t.Run("register new provider", func(t *testing.T) {
		err := RegisterProvider("test", mockFactory)
		assert.NoError(t, err)

		// Verify provider was registered
		provider, err := GetProvider("test")
		assert.NoError(t, err)
		assert.Equal(t, mockFactory, provider)
	})

	t.Run("register duplicate provider", func(t *testing.T) {
		err := RegisterProvider("test", mockFactory)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("case insensitive registration", func(t *testing.T) {
		// Register with uppercase
		err := RegisterProvider("TEST_UPPER", mockFactory)
		assert.NoError(t, err)

		// Try to register with lowercase - should fail
		err = RegisterProvider("test_upper", mockFactory)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})
}

func TestGetProvider(t *testing.T) {
	// Clean up the default registry for testing
	originalProviders := defaultRegistry.providers
	defer func() {
		defaultRegistry.providers = originalProviders
	}()
	defaultRegistry.providers = make(map[string]Factory)

	mockFactory := &MockFactory{}
	RegisterProvider("test", mockFactory)

	t.Run("get existing provider", func(t *testing.T) {
		provider, err := GetProvider("test")
		assert.NoError(t, err)
		assert.Equal(t, mockFactory, provider)
	})

	t.Run("get non-existent provider", func(t *testing.T) {
		provider, err := GetProvider("nonexistent")
		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("case insensitive retrieval", func(t *testing.T) {
		// Register with lowercase
		RegisterProvider("casetest", mockFactory)

		// Retrieve with different cases
		provider1, err1 := GetProvider("casetest")
		provider2, err2 := GetProvider("CASETEST")
		provider3, err3 := GetProvider("CaseTest")

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NoError(t, err3)
		assert.Equal(t, mockFactory, provider1)
		assert.Equal(t, mockFactory, provider2)
		assert.Equal(t, mockFactory, provider3)
	})
}

func TestCreateModel(t *testing.T) {
	// Clean up the default registry for testing
	originalProviders := defaultRegistry.providers
	originalModels := defaultRegistry.models
	defer func() {
		defaultRegistry.providers = originalProviders
		defaultRegistry.models = originalModels
	}()
	defaultRegistry.providers = make(map[string]Factory)
	defaultRegistry.models = make(map[string]Model)

	mockFactory := &MockFactory{}
	mockModel := &MockModel{}

	config := ModelConfig{
		Provider: "test",
		Model:    "test-model",
		APIKey:   "test-key",
	}

	// Register provider and set up mock
	RegisterProvider("test", mockFactory)
	mockFactory.On("CreateModel", config).Return(mockModel, nil)

	t.Run("create new model", func(t *testing.T) {
		model, err := CreateModel(config)
		assert.NoError(t, err)
		assert.Equal(t, mockModel, model)

		// Verify model was cached
		cachedModel, err := GetModel("test", "test-model")
		assert.NoError(t, err)
		assert.Equal(t, mockModel, cachedModel)
	})

	t.Run("get cached model", func(t *testing.T) {
		// Second call should return cached model without calling factory
		model, err := CreateModel(config)
		assert.NoError(t, err)
		assert.Equal(t, mockModel, model)

		// Verify factory was called only once
		mockFactory.AssertExpectations(t)
	})

	t.Run("provider not found", func(t *testing.T) {
		invalidConfig := ModelConfig{
			Provider: "nonexistent",
			Model:    "test-model",
		}

		model, err := CreateModel(invalidConfig)
		assert.Error(t, err)
		assert.Nil(t, model)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGetModel(t *testing.T) {
	// Clean up the default registry for testing
	originalModels := defaultRegistry.models
	defer func() {
		defaultRegistry.models = originalModels
	}()
	defaultRegistry.models = make(map[string]Model)

	mockModel := &MockModel{}
	modelKey := "test:test-model"
	defaultRegistry.models[modelKey] = mockModel

	t.Run("get existing model", func(t *testing.T) {
		model, err := GetModel("test", "test-model")
		assert.NoError(t, err)
		assert.Equal(t, mockModel, model)
	})

	t.Run("get non-existent model", func(t *testing.T) {
		model, err := GetModel("nonexistent", "model")
		assert.Error(t, err)
		assert.Nil(t, model)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestListProviders(t *testing.T) {
	// Clean up the default registry for testing
	originalProviders := defaultRegistry.providers
	defer func() {
		defaultRegistry.providers = originalProviders
	}()
	defaultRegistry.providers = make(map[string]Factory)

	mockFactory1 := &MockFactory{}
	mockFactory2 := &MockFactory{}

	RegisterProvider("provider1", mockFactory1)
	RegisterProvider("provider2", mockFactory2)

	providers := ListProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, "provider1")
	assert.Contains(t, providers, "provider2")
}

func TestListModels(t *testing.T) {
	// Clean up the default registry for testing
	originalModels := defaultRegistry.models
	defer func() {
		defaultRegistry.models = originalModels
	}()
	defaultRegistry.models = make(map[string]Model)

	mockModel1 := &MockModel{}
	mockModel2 := &MockModel{}

	defaultRegistry.models["provider1:model1"] = mockModel1
	defaultRegistry.models["provider2:model2"] = mockModel2

	models := ListModels()
	assert.Len(t, models, 2)
	assert.Contains(t, models, "provider1:model1")
	assert.Contains(t, models, "provider2:model2")
}

func TestParseModelString(t *testing.T) {
	tests := []struct {
		name           string
		modelString    string
		expectProvider string
		expectModel    string
		expectError    bool
	}{
		{
			name:           "valid provider:model format",
			modelString:    "openai:gpt-4",
			expectProvider: "openai",
			expectModel:    "gpt-4",
			expectError:    false,
		},
		{
			name:           "anthropic model",
			modelString:    "anthropic:claude-3-5-sonnet",
			expectProvider: "anthropic",
			expectModel:    "claude-3-5-sonnet",
			expectError:    false,
		},
		{
			name:           "ollama model",
			modelString:    "ollama:llama2",
			expectProvider: "ollama",
			expectModel:    "llama2",
			expectError:    false,
		},
		{
			name:           "model without provider",
			modelString:    "gpt-4",
			expectProvider: "",
			expectModel:    "",
			expectError:    true,
		},
		{
			name:           "empty string",
			modelString:    "",
			expectProvider: "",
			expectModel:    "",
			expectError:    true,
		},
		{
			name:           "only colon",
			modelString:    ":",
			expectProvider: "",
			expectModel:    "",
			expectError:    true,
		},
		{
			name:           "provider without model",
			modelString:    "openai:",
			expectProvider: "",
			expectModel:    "",
			expectError:    true,
		},
		{
			name:           "model without provider (colon at start)",
			modelString:    ":gpt-4",
			expectProvider: "",
			expectModel:    "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, model, err := ParseModelString(tt.modelString)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, provider)
				assert.Empty(t, model)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectProvider, provider)
				assert.Equal(t, tt.expectModel, model)
			}
		})
	}
}

func TestRegistryThreadSafety(t *testing.T) {
	// This test ensures the registry can handle concurrent operations
	// Clean up the default registry for testing
	originalProviders := defaultRegistry.providers
	originalModels := defaultRegistry.models
	defer func() {
		defaultRegistry.providers = originalProviders
		defaultRegistry.models = originalModels
	}()
	defaultRegistry.providers = make(map[string]Factory)
	defaultRegistry.models = make(map[string]Model)

	mockFactory := &MockFactory{}

	// Test concurrent provider registration
	done := make(chan bool, 2)

	go func() {
		err := RegisterProvider("concurrent1", mockFactory)
		assert.NoError(t, err)
		done <- true
	}()

	go func() {
		err := RegisterProvider("concurrent2", mockFactory)
		assert.NoError(t, err)
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify both providers were registered
	provider1, err1 := GetProvider("concurrent1")
	provider2, err2 := GetProvider("concurrent2")

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, mockFactory, provider1)
	assert.Equal(t, mockFactory, provider2)
}

func TestModelCaching(t *testing.T) {
	// Clean up the default registry for testing
	originalProviders := defaultRegistry.providers
	originalModels := defaultRegistry.models
	defer func() {
		defaultRegistry.providers = originalProviders
		defaultRegistry.models = originalModels
	}()
	defaultRegistry.providers = make(map[string]Factory)
	defaultRegistry.models = make(map[string]Model)

	mockFactory := &MockFactory{}
	mockModel := &MockModel{}

	config := ModelConfig{
		Provider: "test",
		Model:    "cache-test",
		APIKey:   "test-key",
	}

	RegisterProvider("test", mockFactory)

	// Set up mock to be called only once
	mockFactory.On("CreateModel", config).Return(mockModel, nil).Once()

	// First call should create the model
	model1, err1 := CreateModel(config)
	assert.NoError(t, err1)
	assert.Equal(t, mockModel, model1)

	// Second call should return cached model
	model2, err2 := CreateModel(config)
	assert.NoError(t, err2)
	assert.Equal(t, mockModel, model2)

	// Verify they're the same instance
	assert.True(t, model1 == model2)

	// Verify factory was called only once
	mockFactory.AssertExpectations(t)
}

func TestModelKeyGeneration(t *testing.T) {
	tests := []struct {
		provider    string
		model       string
		expectedKey string
	}{
		{"openai", "gpt-4", "openai:gpt-4"},
		{"anthropic", "claude-3", "anthropic:claude-3"},
		{"ollama", "llama2", "ollama:llama2"},
		{"provider", "model-with-dashes", "provider:model-with-dashes"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedKey, func(t *testing.T) {
			// Create a mock model and add it to registry
			mockModel := &MockModel{}
			defaultRegistry.models[tt.expectedKey] = mockModel

			// Retrieve using provider and model
			retrievedModel, err := GetModel(tt.provider, tt.model)
			assert.NoError(t, err)
			assert.Equal(t, mockModel, retrievedModel)

			// Clean up
			delete(defaultRegistry.models, tt.expectedKey)
		})
	}
}
