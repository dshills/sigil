package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ProcessManager manages MCP server processes
type ProcessManager struct {
	servers map[string]*ManagedServer
	mu      sync.RWMutex
}

// ManagedServer represents a managed MCP server instance
type ManagedServer struct {
	Name      string
	Config    ServerConfig
	Transport Transport
	Protocol  *ProtocolHandler

	startTime    time.Time
	restartCount int
	lastError    error
	mu           sync.RWMutex
}

// ServerConfig defines configuration for an MCP server
type ServerConfig struct {
	Name        string            `yaml:"name" json:"name"`
	Command     string            `yaml:"command" json:"command"`
	Args        []string          `yaml:"args" json:"args"`
	Env         map[string]string `yaml:"env" json:"env"`
	Transport   string            `yaml:"transport" json:"transport"`
	WorkingDir  string            `yaml:"workingDir" json:"workingDir"`
	AutoRestart bool              `yaml:"autoRestart" json:"autoRestart"`
	MaxRestarts int               `yaml:"maxRestarts" json:"maxRestarts"`
	Settings    struct {
		Timeout    string `yaml:"timeout" json:"timeout"`
		MaxRetries int    `yaml:"maxRetries" json:"maxRetries"`
	} `yaml:"settings" json:"settings"`
}

// NewProcessManager creates a new process manager
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		servers: make(map[string]*ManagedServer),
	}
}

// StartServer starts an MCP server
func (pm *ProcessManager) StartServer(ctx context.Context, config ServerConfig) (*ManagedServer, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if server already exists
	if _, exists := pm.servers[config.Name]; exists {
		return nil, fmt.Errorf("server %s already exists", config.Name)
	}

	// Create transport based on type
	transport, err := pm.createTransport(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Create protocol handler
	protocol := NewProtocolHandler(transport)
	transport.(*StdioTransport).SetMessageHandler(protocol.ProcessMessage)

	// Create managed server
	server := &ManagedServer{
		Name:      config.Name,
		Config:    config,
		Transport: transport,
		Protocol:  protocol,
		startTime: time.Now(),
	}

	// Connect transport
	if err := transport.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect transport: %w", err)
	}

	// Initialize protocol
	clientInfo := ClientInfo{
		Name:    "sigil",
		Version: "1.0.0",
	}

	capabilities := ClientCapabilities{
		Streaming: false,
		Tools:     true,
		Resources: true,
	}

	initResult, err := protocol.Initialize(clientInfo, capabilities)
	if err != nil {
		transport.Close()
		return nil, fmt.Errorf("failed to initialize protocol: %w", err)
	}

	// Log server info
	fmt.Printf("Started MCP server %s (%s %s)\n",
		config.Name,
		initResult.ServerInfo.Name,
		initResult.ServerInfo.Version)

	// Store server
	pm.servers[config.Name] = server

	// Start health monitoring if auto-restart is enabled
	if config.AutoRestart {
		go pm.monitorHealth(server)
	}

	return server, nil
}

// StopServer stops an MCP server
func (pm *ProcessManager) StopServer(name string) error {
	pm.mu.Lock()
	server, exists := pm.servers[name]
	if !exists {
		pm.mu.Unlock()
		return fmt.Errorf("server %s not found", name)
	}
	delete(pm.servers, name)
	pm.mu.Unlock()

	// Shutdown protocol
	if server.Protocol.IsInitialized() {
		if err := server.Protocol.Shutdown(); err != nil {
			// Log error but continue with transport close
			server.mu.Lock()
			server.lastError = err
			server.mu.Unlock()
		}
	}

	// Close transport
	return server.Transport.Close()
}

// GetServer returns a managed server by name
func (pm *ProcessManager) GetServer(name string) (*ManagedServer, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	server, exists := pm.servers[name]
	if !exists {
		return nil, fmt.Errorf("server %s not found", name)
	}

	return server, nil
}

// ListServers returns all managed servers
func (pm *ProcessManager) ListServers() []*ManagedServer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	servers := make([]*ManagedServer, 0, len(pm.servers))
	for _, server := range pm.servers {
		servers = append(servers, server)
	}

	return servers
}

// StopAll stops all managed servers
func (pm *ProcessManager) StopAll() {
	pm.mu.Lock()
	servers := make([]*ManagedServer, 0, len(pm.servers))
	for _, server := range pm.servers {
		servers = append(servers, server)
	}
	pm.mu.Unlock()

	// Stop servers outside of lock
	for _, server := range servers {
		pm.StopServer(server.Name)
	}
}

// createTransport creates a transport based on configuration
func (pm *ProcessManager) createTransport(config ServerConfig) (Transport, error) {
	// Parse timeout
	timeout := 30 * time.Second
	if config.Settings.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(config.Settings.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
	}

	// Create transport config
	transportConfig := TransportConfig{
		Timeout:    timeout,
		MaxRetries: config.Settings.MaxRetries,
		RetryDelay: time.Second,
		BufferSize: 4096,
	}

	if transportConfig.MaxRetries == 0 {
		transportConfig.MaxRetries = 3
	}

	// Expand environment variables
	env := make([]string, 0, len(config.Env))
	for k, v := range config.Env {
		// Expand ${VAR} style variables
		v = os.ExpandEnv(v)
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	switch strings.ToLower(config.Transport) {
	case "stdio", "":
		return NewStdioTransport(config.Command, config.Args, env, transportConfig), nil

	case "sse":
		return nil, fmt.Errorf("SSE transport not yet implemented")

	case "websocket":
		return nil, fmt.Errorf("WebSocket transport not yet implemented")

	default:
		return nil, fmt.Errorf("unknown transport type: %s", config.Transport)
	}
}

// monitorHealth monitors server health and restarts if needed
func (pm *ProcessManager) monitorHealth(server *ManagedServer) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !server.Transport.IsConnected() {
			server.mu.Lock()
			server.lastError = fmt.Errorf("server disconnected")
			restartCount := server.restartCount
			maxRestarts := server.Config.MaxRestarts
			server.mu.Unlock()

			if maxRestarts == 0 {
				maxRestarts = 3
			}

			if restartCount >= maxRestarts {
				fmt.Printf("Server %s exceeded max restarts (%d), not restarting\n",
					server.Name, maxRestarts)
				pm.StopServer(server.Name)
				return
			}

			fmt.Printf("Server %s disconnected, attempting restart...\n", server.Name)

			// Attempt restart
			ctx := context.Background()
			if err := pm.restartServer(ctx, server); err != nil {
				fmt.Printf("Failed to restart server %s: %v\n", server.Name, err)
				server.mu.Lock()
				server.lastError = err
				server.restartCount++
				server.mu.Unlock()
			}
		}
	}
}

// restartServer attempts to restart a server
func (pm *ProcessManager) restartServer(ctx context.Context, server *ManagedServer) error {
	// Close existing transport
	server.Transport.Close()

	// Create new transport
	transport, err := pm.createTransport(server.Config)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	// Create new protocol handler
	protocol := NewProtocolHandler(transport)
	transport.(*StdioTransport).SetMessageHandler(protocol.ProcessMessage)

	// Connect transport
	if err := transport.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect transport: %w", err)
	}

	// Initialize protocol
	clientInfo := ClientInfo{
		Name:    "sigil",
		Version: "1.0.0",
	}

	capabilities := ClientCapabilities{
		Streaming: false,
		Tools:     true,
		Resources: true,
	}

	_, err = protocol.Initialize(clientInfo, capabilities)
	if err != nil {
		transport.Close()
		return fmt.Errorf("failed to initialize protocol: %w", err)
	}

	// Update server
	server.mu.Lock()
	server.Transport = transport
	server.Protocol = protocol
	server.restartCount++
	server.startTime = time.Now()
	server.mu.Unlock()

	return nil
}

// GetStatus returns the status of a server
func (s *ManagedServer) GetStatus() (bool, time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	connected := s.Transport.IsConnected()
	uptime := time.Since(s.startTime)

	return connected, uptime, s.lastError
}
