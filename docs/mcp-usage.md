# Using MCP Servers with Sigil

Model Context Protocol (MCP) servers extend Sigil's capabilities by providing access to tools, resources, and external models through a standardized protocol.

## Quick Start

### 1. Create MCP Server Configuration

```bash
# Create example configuration
sigil mcp init

# Or for global configuration
sigil mcp init --global
```

This creates a `mcp-servers.yml` file with example server configurations.

### 2. Configure Your Servers

Edit `.sigil/mcp-servers.yml`:

```yaml
servers:
  - name: github-mcp
    command: npx
    args: [-y, "@modelcontextprotocol/server-github"]
    env:
      GITHUB_TOKEN: ${GITHUB_TOKEN}
    transport: stdio
    auto_restart: true
```

### 3. Use MCP Servers

```bash
# List configured servers
sigil mcp list

# Use an MCP server with Sigil commands
sigil ask --model mcp://github-mcp "What are the open issues in this repo?"

# Chain multiple servers
sigil edit --model mcp://github-mcp,mcp://postgres-mcp "Update the schema based on latest PRs"
```

## MCP Commands

### List Servers
```bash
sigil mcp list
```
Shows all configured MCP servers.

### Start Server
```bash
sigil mcp start <server-name>
```
Manually start an MCP server (servers auto-start when used).

### Stop Server
```bash
sigil mcp stop <server-name>
```
Stop a running MCP server.

### Server Status
```bash
sigil mcp status
```
Show status of all running servers.

### Initialize Configuration
```bash
sigil mcp init [--global]
```
Create an example MCP server configuration file.

## Configuration Reference

### Server Configuration

```yaml
servers:
  - name: <unique-name>           # Server identifier
    command: <executable>         # Command to run
    args: [<arg1>, <arg2>]       # Command arguments
    env:                         # Environment variables
      KEY: value
      TOKEN: ${ENV_VAR}          # Use ${} for env expansion
    transport: stdio             # Transport type (stdio, sse, websocket)
    working_dir: /path/to/dir    # Working directory
    auto_restart: true           # Restart on failure
    max_restarts: 3              # Maximum restart attempts
    settings:
      timeout: 30s               # Request timeout
      max_retries: 3             # Request retry count
```

### Environment Variables

Environment variables in the configuration are expanded:
- `${GITHUB_TOKEN}` → Value of GITHUB_TOKEN env var
- Undefined variables expand to empty string

### Transport Types

Currently supported:
- **stdio**: Process-based communication (default)

Planned:
- **sse**: Server-Sent Events
- **websocket**: WebSocket connection

## Using MCP Models

### Direct Usage

```bash
# Use specific MCP server
sigil ask --model mcp://server-name "Your prompt"

# Specify model on server
sigil ask --model mcp://server-name/model-name "Your prompt"
```

### In Configuration

Add to `.sigil/config.yml`:

```yaml
models:
  lead: mcp://github-mcp/claude-3-sonnet
  reviewers:
    - mcp://code-review-server/gpt-4
```

### With Options

```bash
sigil ask --model mcp://server-name \
  --option timeout=60s \
  --option env.DEBUG=true \
  "Your prompt"
```

## Available MCP Servers

### Official Servers

1. **GitHub MCP Server**
   ```yaml
   - name: github-mcp
     command: npx
     args: [-y, "@modelcontextprotocol/server-github"]
     env:
       GITHUB_TOKEN: ${GITHUB_TOKEN}
   ```

2. **PostgreSQL MCP Server**
   ```yaml
   - name: postgres-mcp
     command: mcp-server-postgres
     args: ["--database", "mydb"]
     env:
       DATABASE_URL: ${DATABASE_URL}
   ```

### Custom Servers

You can create custom MCP servers in any language. See the [MCP specification](https://modelcontextprotocol.io) for details.

## Troubleshooting

### Server Won't Start

1. Check the command exists:
   ```bash
   which <command>
   ```

2. Verify environment variables are set:
   ```bash
   echo $GITHUB_TOKEN
   ```

3. Check server logs:
   ```bash
   sigil mcp status
   ```

### Connection Errors

1. Ensure server is running:
   ```bash
   sigil mcp status
   ```

2. Check timeout settings:
   ```yaml
   settings:
     timeout: 60s  # Increase for slow servers
   ```

3. Verify transport type matches server

### Authentication Issues

1. Set required environment variables
2. Check server-specific auth requirements
3. Verify API keys/tokens are valid

## Examples

### Code Review with GitHub

```bash
# Review recent PRs
sigil ask --model mcp://github-mcp \
  "Summarize the changes in the last 5 PRs"

# Analyze codebase
sigil explain --model mcp://github-mcp \
  --file src/main.go \
  "How does this relate to recent issues?"
```

### Database Operations

```bash
# Generate migration
sigil ask --model mcp://postgres-mcp \
  "Generate a migration to add user_roles table"

# Analyze schema
sigil doc --model mcp://postgres-mcp \
  "Document the database schema"
```

### Multi-Server Workflows

```bash
# Code generation based on DB schema
sigil edit --model mcp://postgres-mcp,mcp://github-mcp \
  --file models/user.go \
  "Update the User model to match the database schema"
```

## Security Considerations

1. **Environment Variables**: Store sensitive tokens in environment variables, not in config files
2. **Server Processes**: MCP servers run as separate processes with access to your system
3. **Network Access**: Some servers may make network requests
4. **File Access**: Servers may have file system access based on their implementation

## Advanced Features

### Connection Pooling

Sigil automatically manages connection pools for better performance:

```yaml
# Configure in .sigil/config.yml
mcp:
  connection_pool:
    default_size: 3        # Default connections per server
    max_size: 10          # Maximum connections per server
    cleanup_interval: 5m  # Pool cleanup frequency
```

### Health Monitoring

Automatic health monitoring with restart capabilities:

```yaml
mcp:
  health_monitoring:
    enabled: true
    check_interval: 15s      # How often to check server health
    failure_threshold: 3     # Failures before marking unhealthy
    recovery_timeout: 2m     # Time before attempting restart
```

### Tool Calling

Use MCP server tools directly:

```bash
# List available tools from a server
sigil mcp status github-mcp --tools

# Tools are automatically available when using MCP models
sigil ask --model mcp://github-mcp "Create a new issue for bug fix"
```

### Resource Management

Access server-provided resources:

```bash
# List available resources
sigil mcp status postgres-mcp --resources

# Resources provide context automatically
sigil explain schema.sql --model mcp://postgres-mcp
```

### Prompt Templates

Use server-provided prompt templates:

```bash
# List available prompt templates
sigil mcp status python-tools --prompts

# Templates enhance model capabilities
sigil edit mycode.py --model mcp://python-tools --template "optimize_code"
```

## Performance Tips

1. **Server Reuse**: Servers stay running between commands for better performance
2. **Timeouts**: Adjust timeouts based on server response times
3. **Auto-restart**: Disable for development servers that change frequently
4. **Connection Pooling**: Servers automatically pool connections for efficiency
5. **Health Monitoring**: Failed servers restart automatically for reliability

## Troubleshooting

### Common Issues

1. **Server Won't Start**
   ```bash
   # Check server logs
   sigil mcp status server-name --verbose
   
   # Manually test server command
   npx -y @modelcontextprotocol/server-github
   ```

2. **Connection Timeouts**
   ```yaml
   # Increase timeouts in server config
   settings:
     timeout: 60s
     max_retries: 5
   ```

3. **Server Crashes**
   ```yaml
   # Enable auto-restart with more attempts
   auto_restart: true
   max_restarts: 10
   ```

### Debug Mode

Enable verbose logging for troubleshooting:

```bash
# Enable verbose output
sigil --verbose mcp status

# Check server health
sigil mcp status --json | jq '.servers[] | {name, status, error}'
```

## Next Steps

- Explore available [MCP servers](https://github.com/topics/mcp-server)
- Create your own [custom MCP server](https://modelcontextprotocol.io/docs/create-server)
- Check out the example configurations in `examples/config/mcp-servers.yaml`
- Run the demo script: `examples/scripts/mcp-demo.sh`
- Read the [MCP specification](https://modelcontextprotocol.io/spec)