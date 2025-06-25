package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProcessManager(t *testing.T) {
	pm := NewProcessManager()
	require.NotNil(t, pm)
	assert.NotNil(t, pm.servers)
	assert.NotNil(t, pm.connectionPool)
	assert.Equal(t, 3, pm.poolSize)
	assert.NotNil(t, pm.ctx)
	assert.NotNil(t, pm.cancel)
	assert.NotNil(t, pm.healthTicker)

	// Clean up
	pm.StopAll()
}

func TestProcessManager_SetPoolSize(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	pm.SetPoolSize(5)
	assert.Equal(t, 5, pm.poolSize)
}

func TestProcessManager_ServerLifecycle(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	config := ServerConfig{
		Name:        "test-server",
		Command:     "echo",
		Args:        []string{"hello"},
		Transport:   "stdio",
		AutoRestart: false,
		MaxRestarts: 3,
	}

	ctx := context.Background()

	// Test starting a server
	server, err := pm.StartServer(ctx, config)
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.Equal(t, "test-server", server.Name)
	assert.Equal(t, config, server.Config)

	// Test getting the server
	retrievedServer, err := pm.GetServer("test-server")
	require.NoError(t, err)
	assert.Equal(t, server, retrievedServer)

	// Test listing servers
	servers := pm.ListServers()
	assert.Len(t, servers, 1)
	assert.Equal(t, server, servers[0])

	// Test server status
	status := server.GetStatus()
	assert.Equal(t, "test-server", status.Name)
	assert.Greater(t, status.Uptime, time.Duration(0))

	// Test stopping the server
	err = pm.StopServer("test-server")
	require.NoError(t, err)

	// Verify server is removed
	_, err = pm.GetServer("test-server")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProcessManager_DuplicateServer(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	config := ServerConfig{
		Name:      "test-server",
		Command:   "echo",
		Transport: "stdio",
	}

	ctx := context.Background()

	// Start first server
	_, err := pm.StartServer(ctx, config)
	require.NoError(t, err)

	// Try to start duplicate server
	_, err = pm.StartServer(ctx, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestProcessManager_StopNonexistentServer(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	err := pm.StopServer("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProcessManager_ConnectionPooling(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	config := ServerConfig{
		Name:      "pool-test",
		Command:   "echo",
		Transport: "stdio",
	}

	ctx := context.Background()

	// Start main server
	server, err := pm.StartServer(ctx, config)
	require.NoError(t, err)

	// Test getting pooled connection
	pooledConn, err := pm.GetPooledConnection("pool-test")
	require.NoError(t, err)
	assert.NotNil(t, pooledConn)
	assert.Contains(t, pooledConn.Name, "pool-test-pool-")

	// Check connection is in use
	status := pooledConn.GetStatus()
	assert.True(t, status.InUse)
	assert.Equal(t, int64(1), status.RequestCount)

	// Release connection
	pm.ReleaseConnection(pooledConn)

	// Check connection is released
	status = pooledConn.GetStatus()
	assert.False(t, status.InUse)

	// Test pool status
	poolStatus := pm.GetPoolStatus()
	assert.Contains(t, poolStatus, "pool-test")
	assert.Len(t, poolStatus["pool-test"], 1)

	// Clean up the server
	pm.StopServer(server.Name)
}

func TestProcessManager_PoolLimits(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	pm.SetPoolSize(2) // Set small pool size for testing

	config := ServerConfig{
		Name:      "limit-test",
		Command:   "echo",
		Transport: "stdio",
	}

	ctx := context.Background()

	// Start main server
	_, err := pm.StartServer(ctx, config)
	require.NoError(t, err)

	// Get connections up to pool limit
	conn1, err := pm.GetPooledConnection("limit-test")
	require.NoError(t, err)

	conn2, err := pm.GetPooledConnection("limit-test")
	require.NoError(t, err)

	// Try to get another connection (should fail due to pool limit)
	_, err = pm.GetPooledConnection("limit-test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pool is full")

	// Release one connection
	pm.ReleaseConnection(conn1)

	// Now should be able to get another connection
	conn3, err := pm.GetPooledConnection("limit-test")
	require.NoError(t, err)
	assert.NotNil(t, conn3)

	// Clean up
	pm.ReleaseConnection(conn2)
	pm.ReleaseConnection(conn3)
}

func TestProcessManager_OverallHealth(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	// Test empty health metrics
	health := pm.GetOverallHealth()
	assert.Equal(t, 0, health["totalServers"])
	assert.Equal(t, 0, health["connectedServers"])
	assert.Equal(t, 0, health["pooledConnections"])
	assert.Equal(t, int64(0), health["totalRequests"])

	// Start a server
	config := ServerConfig{
		Name:      "health-test",
		Command:   "echo",
		Transport: "stdio",
	}

	ctx := context.Background()
	server, err := pm.StartServer(ctx, config)
	require.NoError(t, err)

	// Get a pooled connection
	pooledConn, err := pm.GetPooledConnection("health-test")
	require.NoError(t, err)

	// Check health metrics
	health = pm.GetOverallHealth()
	assert.Equal(t, 1, health["totalServers"])
	assert.GreaterOrEqual(t, health["connectedServers"], 0) // May vary based on actual connection status
	assert.Equal(t, 1, health["pooledConnections"])
	assert.GreaterOrEqual(t, health["totalRequests"], int64(1))

	// Clean up
	pm.ReleaseConnection(pooledConn)
	pm.StopServer(server.Name)
}

func TestProcessManager_TransportConfig(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	config := ServerConfig{
		Name:      "config-test",
		Command:   "echo",
		Transport: "stdio",
		Settings: struct {
			Timeout    string `yaml:"timeout" json:"timeout"`
			MaxRetries int    `yaml:"maxRetries" json:"maxRetries"`
		}{
			Timeout:    "30s",
			MaxRetries: 5,
		},
		Env: map[string]string{
			"TEST_VAR": "${HOME}/test",
		},
	}

	// Test transport creation (internal method)
	transport, err := pm.createTransport(config)
	require.NoError(t, err)
	assert.NotNil(t, transport)

	// Test with invalid timeout
	invalidConfig := config
	invalidConfig.Settings.Timeout = "invalid"

	_, err = pm.createTransport(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout")

	// Test with unsupported transport
	unsupportedConfig := config
	unsupportedConfig.Transport = "unknown"

	_, err = pm.createTransport(unsupportedConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown transport type")
}

func TestServerStatus(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	config := ServerConfig{
		Name:      "status-test",
		Command:   "echo",
		Transport: "stdio",
	}

	ctx := context.Background()
	server, err := pm.StartServer(ctx, config)
	require.NoError(t, err)

	// Test status immediately after creation
	status := server.GetStatus()
	assert.Equal(t, "status-test", status.Name)
	assert.Greater(t, status.Uptime, time.Duration(0))
	assert.Equal(t, 0, status.RestartCount)
	assert.Empty(t, status.LastError)
	assert.False(t, status.InUse)
	assert.Equal(t, int64(0), status.RequestCount)

	// Simulate some activity
	server.mu.Lock()
	server.requestCount = 5
	server.inUse = true
	server.lastError = assert.AnError
	server.mu.Unlock()

	status = server.GetStatus()
	assert.Equal(t, int64(5), status.RequestCount)
	assert.True(t, status.InUse)
	assert.Equal(t, assert.AnError.Error(), status.LastError)

	// Clean up
	pm.StopServer(server.Name)
}

func TestProcessManager_StopAll(t *testing.T) {
	pm := NewProcessManager()

	// Start multiple servers
	configs := []ServerConfig{
		{Name: "server1", Command: "echo", Transport: "stdio"},
		{Name: "server2", Command: "echo", Transport: "stdio"},
		{Name: "server3", Command: "echo", Transport: "stdio"},
	}

	ctx := context.Background()
	for _, config := range configs {
		_, err := pm.StartServer(ctx, config)
		require.NoError(t, err)
	}

	// Get some pooled connections
	_, err := pm.GetPooledConnection("server1")
	require.NoError(t, err)
	_, err = pm.GetPooledConnection("server2")
	require.NoError(t, err)

	// Verify servers are running
	assert.Len(t, pm.ListServers(), 3)
	poolStatus := pm.GetPoolStatus()
	assert.GreaterOrEqual(t, len(poolStatus), 2)

	// Stop all
	pm.StopAll()

	// Verify everything is cleaned up
	assert.Len(t, pm.ListServers(), 0)
	poolStatus = pm.GetPoolStatus()
	assert.Len(t, poolStatus, 0)
}

func TestProcessManager_HealthMonitoring(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	// Create a server that will be used for health monitoring
	config := ServerConfig{
		Name:        "health-monitor-test",
		Command:     "echo",
		Transport:   "stdio",
		AutoRestart: true,
		MaxRestarts: 2,
	}

	ctx := context.Background()
	server, err := pm.StartServer(ctx, config)
	require.NoError(t, err)

	// Check initial health check time
	initialTime := server.lastHealthCheck

	// Wait a bit and trigger health check
	time.Sleep(100 * time.Millisecond)
	pm.checkServerHealth(server)

	// Verify health check time was updated
	assert.True(t, server.lastHealthCheck.After(initialTime))

	// Clean up
	pm.StopServer(server.Name)
}

func TestProcessManager_NonexistentPoolConnection(t *testing.T) {
	pm := NewProcessManager()
	defer pm.StopAll()

	_, err := pm.GetPooledConnection("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
