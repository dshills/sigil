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
	servers       map[string]*ManagedServer
	connectionPool map[string][]*ManagedServer
	poolSize      int
	healthTicker  *time.Ticker
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	poolMu        sync.RWMutex
}

// ManagedServer represents a managed MCP server instance
type ManagedServer struct {
	Name      string
	Config    ServerConfig
	Transport Transport
	Protocol  *ProtocolHandler

	startTime       time.Time
	restartCount    int
	lastError       error
	lastHealthCheck time.Time
	requestCount    int64
	inUse          bool
	mu             sync.RWMutex
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
	ctx, cancel := context.WithCancel(context.Background())
	pm := &ProcessManager{
		servers:        make(map[string]*ManagedServer),
		connectionPool: make(map[string][]*ManagedServer),
		poolSize:       3, // Default pool size
		ctx:            ctx,
		cancel:         cancel,
	}

	// Start global health monitoring
	pm.healthTicker = time.NewTicker(15 * time.Second)
	go pm.globalHealthMonitor()

	return pm
}

// SetPoolSize sets the connection pool size
func (pm *ProcessManager) SetPoolSize(size int) {
	pm.poolMu.Lock()
	defer pm.poolMu.Unlock()
	pm.poolSize = size
}

// GetPooledConnection gets a connection from the pool or creates a new one
func (pm *ProcessManager) GetPooledConnection(serverName string) (*ManagedServer, error) {
	pm.poolMu.Lock()
	defer pm.poolMu.Unlock()

	// Check for available connections in pool
	if connections, exists := pm.connectionPool[serverName]; exists {
		for i, conn := range connections {
			if !conn.inUse {
				conn.mu.Lock()
				conn.inUse = true
				conn.requestCount++
				conn.mu.Unlock()
				
				// Move connection to end of slice (LRU)
				if i < len(connections)-1 {
					connections[i], connections[len(connections)-1] = connections[len(connections)-1], connections[i]
				}
				
				return conn, nil
			}
		}
	}

	// No available connections, check if we can create a new one
	pm.mu.RLock()
	server, exists := pm.servers[serverName]
	pm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server %s not found", serverName)
	}

	// Check pool size limit
	if len(pm.connectionPool[serverName]) >= pm.poolSize {
		return nil, fmt.Errorf("connection pool for server %s is full", serverName)
	}

	// Create new pooled connection
	pooledServer, err := pm.createPooledConnection(server.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pooled connection: %w", err)
	}

	// Add to pool
	if pm.connectionPool[serverName] == nil {
		pm.connectionPool[serverName] = make([]*ManagedServer, 0, pm.poolSize)
	}
	pm.connectionPool[serverName] = append(pm.connectionPool[serverName], pooledServer)

	pooledServer.mu.Lock()
	pooledServer.inUse = true
	pooledServer.requestCount++
	pooledServer.mu.Unlock()

	return pooledServer, nil
}

// ReleaseConnection releases a pooled connection back to the pool
func (pm *ProcessManager) ReleaseConnection(server *ManagedServer) {
	server.mu.Lock()
	server.inUse = false
	server.mu.Unlock()
}

// createPooledConnection creates a new connection for the pool
func (pm *ProcessManager) createPooledConnection(config ServerConfig) (*ManagedServer, error) {
	// Create transport
	transport, err := pm.createTransport(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Create protocol handler
	protocol := NewProtocolHandler(transport)
	transport.(*StdioTransport).SetMessageHandler(protocol.ProcessMessage)

	// Create managed server
	server := &ManagedServer{
		Name:      fmt.Sprintf("%s-pool-%d", config.Name, time.Now().UnixNano()),
		Config:    config,
		Transport: transport,
		Protocol:  protocol,
		startTime: time.Now(),
	}

	// Connect transport
	if err := transport.Connect(pm.ctx); err != nil {
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

	_, err = protocol.Initialize(clientInfo, capabilities)
	if err != nil {
		transport.Close()
		return nil, fmt.Errorf("failed to initialize protocol: %w", err)
	}

	return server, nil
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
	// Cancel health monitoring
	pm.cancel()
	if pm.healthTicker != nil {
		pm.healthTicker.Stop()
	}

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

	// Clean up connection pools
	pm.poolMu.Lock()
	for serverName, connections := range pm.connectionPool {
		for _, conn := range connections {
			if conn.Protocol.IsInitialized() {
				conn.Protocol.Shutdown()
			}
			conn.Transport.Close()
		}
		delete(pm.connectionPool, serverName)
	}
	pm.poolMu.Unlock()
}

// globalHealthMonitor monitors health of all servers
func (pm *ProcessManager) globalHealthMonitor() {
	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-pm.healthTicker.C:
			pm.performHealthChecks()
		}
	}
}

// performHealthChecks performs health checks on all servers
func (pm *ProcessManager) performHealthChecks() {
	pm.mu.RLock()
	servers := make([]*ManagedServer, 0, len(pm.servers))
	for _, server := range pm.servers {
		servers = append(servers, server)
	}
	pm.mu.RUnlock()

	for _, server := range servers {
		go pm.checkServerHealth(server)
	}

	// Also check pooled connections
	pm.poolMu.RLock()
	for _, connections := range pm.connectionPool {
		for _, conn := range connections {
			go pm.checkServerHealth(conn)
		}
	}
	pm.poolMu.RUnlock()
}

// checkServerHealth performs a health check on a single server
func (pm *ProcessManager) checkServerHealth(server *ManagedServer) {
	server.mu.Lock()
	server.lastHealthCheck = time.Now()
	server.mu.Unlock()

	if !server.Transport.IsConnected() {
		server.mu.Lock()
		server.lastError = fmt.Errorf("server disconnected")
		server.mu.Unlock()

		// Handle disconnection based on server type
		if strings.Contains(server.Name, "-pool-") {
			// This is a pooled connection, remove it from pool
			pm.removeFromPool(server)
		} else if server.Config.AutoRestart {
			// This is a main server, attempt restart
			go pm.attemptRestart(server)
		}
		return
	}

	// Perform ping health check
	if server.Protocol.IsInitialized() {
		_, err := server.Protocol.Ping("health-check")
		if err != nil {
			server.mu.Lock()
			server.lastError = fmt.Errorf("ping failed: %w", err)
			server.mu.Unlock()
		} else {
			server.mu.Lock()
			server.lastError = nil
			server.mu.Unlock()
		}
	}
}

// removeFromPool removes a server from the connection pool
func (pm *ProcessManager) removeFromPool(server *ManagedServer) {
	pm.poolMu.Lock()
	defer pm.poolMu.Unlock()

	for serverName, connections := range pm.connectionPool {
		for i, conn := range connections {
			if conn == server {
				// Close the connection
				if conn.Protocol.IsInitialized() {
					conn.Protocol.Shutdown()
				}
				conn.Transport.Close()

				// Remove from slice
				connections[i] = connections[len(connections)-1]
				pm.connectionPool[serverName] = connections[:len(connections)-1]
				return
			}
		}
	}
}

// attemptRestart attempts to restart a failed server
func (pm *ProcessManager) attemptRestart(server *ManagedServer) {
	server.mu.Lock()
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

	fmt.Printf("Server %s health check failed, attempting restart...\n", server.Name)

	// Attempt restart
	if err := pm.restartServer(pm.ctx, server); err != nil {
		fmt.Printf("Failed to restart server %s: %v\n", server.Name, err)
		server.mu.Lock()
		server.lastError = err
		server.restartCount++
		server.mu.Unlock()
	} else {
		fmt.Printf("Successfully restarted server %s\n", server.Name)
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

// ServerStatus represents the detailed status of a server
type ServerStatus struct {
	Name            string        `json:"name"`
	Connected       bool          `json:"connected"`
	Uptime          time.Duration `json:"uptime"`
	RestartCount    int           `json:"restartCount"`
	LastError       string        `json:"lastError,omitempty"`
	LastHealthCheck time.Time     `json:"lastHealthCheck"`
	RequestCount    int64         `json:"requestCount"`
	InUse           bool          `json:"inUse"`
	Protocol        string        `json:"protocol,omitempty"`
	ServerInfo      *ServerInfo   `json:"serverInfo,omitempty"`
}

// GetStatus returns the detailed status of a server
func (s *ManagedServer) GetStatus() ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := ServerStatus{
		Name:            s.Name,
		Connected:       s.Transport.IsConnected(),
		Uptime:          time.Since(s.startTime),
		RestartCount:    s.restartCount,
		LastHealthCheck: s.lastHealthCheck,
		RequestCount:    s.requestCount,
		InUse:           s.inUse,
	}

	if s.lastError != nil {
		status.LastError = s.lastError.Error()
	}

	if s.Protocol.IsInitialized() {
		status.Protocol = "initialized"
		// Could add more protocol info here
	}

	return status
}

// GetPoolStatus returns the status of all connection pools
func (pm *ProcessManager) GetPoolStatus() map[string][]ServerStatus {
	pm.poolMu.RLock()
	defer pm.poolMu.RUnlock()

	poolStatus := make(map[string][]ServerStatus)
	for serverName, connections := range pm.connectionPool {
		status := make([]ServerStatus, len(connections))
		for i, conn := range connections {
			status[i] = conn.GetStatus()
		}
		poolStatus[serverName] = status
	}

	return poolStatus
}

// GetOverallHealth returns overall health metrics
func (pm *ProcessManager) GetOverallHealth() map[string]interface{} {
	pm.mu.RLock()
	serverCount := len(pm.servers)
	pm.mu.RUnlock()

	pm.poolMu.RLock()
	poolCount := 0
	for _, connections := range pm.connectionPool {
		poolCount += len(connections)
	}
	pm.poolMu.RUnlock()

	connectedServers := 0
	totalRequests := int64(0)

	for _, server := range pm.ListServers() {
		status := server.GetStatus()
		if status.Connected {
			connectedServers++
		}
		totalRequests += status.RequestCount
	}

	return map[string]interface{}{
		"totalServers":     serverCount,
		"connectedServers": connectedServers,
		"pooledConnections": poolCount,
		"totalRequests":    totalRequests,
		"healthCheckInterval": "15s",
	}
}
