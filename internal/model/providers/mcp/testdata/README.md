# MCP Integration Tests

This directory contains configuration and test data for MCP integration tests.

## Configuration

The `integration_config.json` file defines real MCP servers to test against. Each server configuration includes:

- `command`: The command to execute
- `args`: Command arguments
- `env`: Environment variables (supports `${VAR}` expansion)
- `timeout`: Test timeout duration
- `skipIfMissing`: Skip test if command is not available

## Running Integration Tests

### Basic Integration Tests
```bash
# Run all integration tests
go test -v -run TestIntegration

# Run integration tests for real servers only
go test -v -run TestIntegration_RealServers

# Run with longer timeout
go test -v -timeout 5m -run TestIntegration
```

### Skip Integration Tests
```bash
# Skip integration tests (short mode)
go test -short ./...
```

## Test Servers

### Built-in Test Servers
- **echo-json**: Simple echo command that outputs valid JSON-RPC response
- **python-server**: Python script that outputs MCP response
- **node-server**: Node.js script that outputs MCP response

### Real MCP Servers
- **github-mcp**: Official GitHub MCP server (requires `GITHUB_TOKEN`)
- **filesystem-mcp**: Official filesystem MCP server

## Environment Variables

Some MCP servers require environment variables:

```bash
# For GitHub MCP server
export GITHUB_TOKEN="your_github_token"

# Run tests
go test -v -run TestIntegration_RealServers
```

## Adding New Test Servers

To add a new MCP server for testing:

1. Add configuration to `integration_config.json`:
```json
{
  "my-server": {
    "command": "my-mcp-server",
    "args": ["--port", "8080"],
    "env": {
      "API_KEY": "${MY_API_KEY}"
    },
    "timeout": "30s",
    "skipIfMissing": true
  }
}
```

2. Set `skipIfMissing: true` if the server might not be available in all environments.

## Test Coverage

Integration tests cover:

1. **Connectivity**: Basic connection and initialization
2. **Protocol**: MCP protocol operations (ping, capabilities)
3. **Resources**: Listing and reading resources (if supported)
4. **Tools**: Listing and calling tools (if supported)
5. **Error Handling**: Invalid method calls and error responses
6. **Process Management**: Multiple servers, connection pooling
7. **Health Monitoring**: Server status and health checks
8. **Error Recovery**: Reconnection and failure handling

## Safety

Integration tests are designed to be safe:

- Only call read-only or explicitly safe tool operations
- Skip potentially destructive operations
- Use timeouts to prevent hanging
- Gracefully handle missing dependencies
- Clean up resources after tests

## Troubleshooting

### Server Not Found
If you get "server not available" errors:
- Ensure the command is installed and in PATH
- Check that required environment variables are set
- Verify the server configuration is correct

### Timeout Issues
If tests timeout:
- Increase timeout in configuration
- Check server startup time
- Verify server responds correctly to initialization

### Permission Issues
If you get permission errors:
- Ensure the test user has necessary permissions
- Check file/directory access for filesystem servers
- Verify API tokens are valid and have required scopes