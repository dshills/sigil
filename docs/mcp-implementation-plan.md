# MCP (Model Context Protocol) Implementation Plan

## Overview

This document outlines the implementation plan for full MCP server support in Sigil. The Model Context Protocol allows Sigil to communicate with external LLM servers that implement the MCP specification, enabling tool use, resource access, and advanced capabilities.

## Current State

### What Exists
- Basic MCP provider structure in `/internal/model/providers/mcp/`
- Integration with Sigil's model interface
- Configuration structures for MCP settings
- Placeholder implementation returning simulated responses

### What's Missing
- Actual transport layer implementations (stdio, SSE, WebSocket)
- MCP protocol implementation (JSON-RPC 2.0 methods)
- Server lifecycle management
- Tool/function calling support
- Resource management
- Streaming capabilities
- Proper error handling and retry logic

## Implementation Phases

### Phase 1: Core Infrastructure (Week 1-2)

#### 1.1 MCP Protocol Implementation
**Goal**: Implement core MCP protocol methods following the JSON-RPC 2.0 specification.

**Tasks**:
- [ ] Implement protocol handler interface
- [ ] Add message framing for stdio transport
- [ ] Implement core methods:
  - `initialize` - Server initialization handshake
  - `initialized` - Confirmation callback
  - `shutdown` - Graceful shutdown
  - `ping/pong` - Keep-alive mechanism
  - `completion/complete` - Text generation
- [ ] Add proper error handling with MCP error codes
- [ ] Implement request/response correlation

**Files to modify**:
- Create `internal/model/providers/mcp/protocol.go`
- Create `internal/model/providers/mcp/transport.go`
- Update `internal/model/providers/mcp/client.go`

#### 1.2 stdio Transport Implementation
**Goal**: Enable communication with MCP servers via standard input/output.

**Tasks**:
- [ ] Implement process spawning and management
- [ ] Add bidirectional stdio communication
- [ ] Implement message framing (newline-delimited JSON)
- [ ] Add connection state management
- [ ] Handle process lifecycle (start, stop, restart)
- [ ] Implement read/write timeouts

**Files to create**:
- `internal/model/providers/mcp/transport_stdio.go`
- `internal/model/providers/mcp/process_manager.go`

#### 1.3 Server Configuration Schema
**Goal**: Define configuration format for MCP servers.

**Configuration structure**:
```yaml
mcp:
  servers:
    - name: "example-server"
      command: "python"
      args: ["-m", "example_mcp_server"]
      env:
        API_KEY: "${EXAMPLE_API_KEY}"
      transport: "stdio"
      settings:
        timeout: 30s
        max_retries: 3
```

**Tasks**:
- [ ] Extend config structures in `internal/config/config.go`
- [ ] Add server configuration validation
- [ ] Support environment variable expansion
- [ ] Add configuration loading from files

### Phase 2: Core Functionality (Week 3-4)

#### 2.1 Server Lifecycle Management
**Goal**: Robust server process management with automatic recovery.

**Tasks**:
- [ ] Implement server registry for tracking active servers
- [ ] Add automatic server startup on first use
- [ ] Implement health checking and auto-restart
- [ ] Add graceful shutdown on application exit
- [ ] Implement connection pooling for multiple requests
- [ ] Add server status monitoring

**Files to create**:
- `internal/model/providers/mcp/server_manager.go`
- `internal/model/providers/mcp/health_check.go`

#### 2.2 Tool/Function Calling
**Goal**: Enable MCP servers to expose and execute tools.

**Tasks**:
- [ ] Implement tool discovery via `tools/list` method
- [ ] Add tool execution via `tools/call` method
- [ ] Handle tool parameter validation
- [ ] Implement result formatting
- [ ] Add tool caching for performance
- [ ] Integrate with Sigil's command structure

**Files to create**:
- `internal/model/providers/mcp/tools.go`

#### 2.3 Configuration File Support
**Goal**: Allow users to define MCP servers in configuration files.

**Tasks**:
- [ ] Add support for `.sigil/mcp-servers.yml`
- [ ] Implement global MCP server configuration
- [ ] Add per-project server overrides
- [ ] Support multiple configuration formats (YAML, JSON)
- [ ] Add configuration validation and error reporting

### Phase 3: Advanced Features (Week 5-6)

#### 3.1 Additional Transport Implementations
**Goal**: Support SSE and WebSocket transports for different server types.

**SSE (Server-Sent Events)**:
- [ ] Implement HTTP client with SSE support
- [ ] Handle connection establishment and auth
- [ ] Implement event parsing and dispatching
- [ ] Add reconnection logic

**WebSocket**:
- [ ] Implement WebSocket client
- [ ] Handle connection upgrade
- [ ] Implement bidirectional messaging
- [ ] Add ping/pong for connection health

**Files to create**:
- `internal/model/providers/mcp/transport_sse.go`
- `internal/model/providers/mcp/transport_websocket.go`

#### 3.2 Resource Management
**Goal**: Support MCP resource access (files, databases, APIs).

**Tasks**:
- [ ] Implement `resources/list` method
- [ ] Add `resources/read` method
- [ ] Implement resource caching
- [ ] Add resource update notifications
- [ ] Support resource templates

**Files to create**:
- `internal/model/providers/mcp/resources.go`

#### 3.3 Streaming Support
**Goal**: Enable streaming responses for real-time output.

**Tasks**:
- [ ] Implement streaming protocol extensions
- [ ] Add chunked response handling
- [ ] Integrate with Sigil's streaming output
- [ ] Handle partial responses and buffering
- [ ] Add progress reporting

### Phase 4: Testing and Documentation (Week 6-7)

#### 4.1 Testing Infrastructure
**Goal**: Comprehensive test coverage with mock servers.

**Tasks**:
- [ ] Create mock MCP server for testing
- [ ] Add integration tests for each transport
- [ ] Implement protocol compliance tests
- [ ] Add performance benchmarks
- [ ] Create error scenario tests

**Files to create**:
- `internal/model/providers/mcp/mock_server_test.go`
- `internal/model/providers/mcp/integration_test.go`

#### 4.2 Documentation
**Goal**: Complete documentation for users and developers.

**Tasks**:
- [ ] Write user guide for MCP configuration
- [ ] Document supported MCP servers
- [ ] Create developer guide for adding new transports
- [ ] Add troubleshooting guide
- [ ] Include example configurations

## Technical Considerations

### Error Handling
- Implement exponential backoff for retries
- Distinguish between recoverable and fatal errors
- Provide clear error messages for configuration issues
- Log all MCP protocol exchanges for debugging

### Performance
- Implement connection pooling for reuse
- Cache tool and resource definitions
- Use buffered I/O for stdio transport
- Implement request batching where possible

### Security
- Validate all server commands before execution
- Implement process isolation
- Support authentication mechanisms
- Sanitize environment variables
- Restrict file system access for servers

### Compatibility
- Target MCP protocol version 1.0
- Support graceful degradation for missing features
- Implement version negotiation
- Handle protocol extensions

## Success Criteria

1. **Phase 1**: Successfully communicate with at least one MCP server via stdio
2. **Phase 2**: Execute tools and manage server lifecycle reliably
3. **Phase 3**: Support multiple transport types and advanced features
4. **Phase 4**: Pass all integration tests with 90%+ coverage

## Example Usage

### Configuration
```yaml
# .sigil/mcp-servers.yml
servers:
  - name: github-mcp
    command: npx
    args: [-y, @modelcontextprotocol/server-github]
    env:
      GITHUB_TOKEN: ${GITHUB_TOKEN}
    transport: stdio
    
  - name: postgres-mcp
    command: mcp-server-postgres
    args: [--database, mydb]
    transport: stdio
    settings:
      timeout: 60s
```

### CLI Usage
```bash
# Use MCP server for code analysis
sigil explain --model mcp:github-mcp --file main.go

# Execute MCP server tools
sigil ask --model mcp:postgres-mcp "Show me all tables in the database"

# Chain multiple MCP servers
sigil edit --model mcp:github-mcp,mcp:postgres-mcp --prompt "Update the schema based on latest migrations"
```

## Timeline

- **Week 1-2**: Core infrastructure (stdio transport, basic protocol)
- **Week 3-4**: Functionality (lifecycle, tools, configuration)
- **Week 5-6**: Advanced features (additional transports, resources)
- **Week 6-7**: Testing and documentation

Total estimated time: 6-7 weeks for full implementation

## Dependencies

- No additional external dependencies required
- Uses Go's standard library for process management
- Optional: `gorilla/websocket` for WebSocket support
- Optional: `r3labs/sse` for SSE client support

## Risks and Mitigations

1. **Risk**: MCP protocol changes
   - **Mitigation**: Version negotiation and compatibility layer

2. **Risk**: Server process management complexity
   - **Mitigation**: Start with simple cases, add robustness incrementally

3. **Risk**: Performance impact of external processes
   - **Mitigation**: Connection pooling and efficient protocols

4. **Risk**: Security vulnerabilities from running external processes
   - **Mitigation**: Strict validation and sandboxing options