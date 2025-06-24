package config

import (
	"sync"
)

var (
	globalConfig *Config
	configMutex  sync.RWMutex
)

// Get returns the global configuration
func Get() *Config {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if globalConfig == nil {
		// Return default if not initialized
		return Default()
	}

	return globalConfig
}

// Set sets the global configuration
func Set(config *Config) {
	configMutex.Lock()
	defer configMutex.Unlock()

	globalConfig = config
}

// Load loads and sets the global configuration
func Load(configFile string) error {
	loader := NewLoader()
	config, err := loader.Load(configFile)
	if err != nil {
		return err
	}

	Set(config)
	return nil
}
