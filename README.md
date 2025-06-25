# Sigil

Sigil is an intelligent command-line tool for autonomous code transformation using Large Language Models (LLMs). It provides AI-powered assistance for code editing, review, documentation, and analysis while maintaining strict Git integration for safety and version control.

## Features

- **Multi-Model Support**: Works with OpenAI, Anthropic Claude, Ollama (local), and MCP providers
- **Git-Centric**: All operations require Git repository for safety and version control
- **Multi-Agent System**: Orchestrates multiple AI agents for validation and consensus
- **Sandbox Execution**: Validates changes in isolated Git worktrees before applying
- **Memory System**: Maintains context across sessions with Markdown-based storage
- **Flexible Input**: Supports files, directories, Git diffs, and stdin
- **Multiple Output Formats**: JSON, Markdown, HTML, XML, SARIF, patch files

## Installation

```bash
# Clone the repository
git clone https://github.com/dshills/sigil.git
cd sigil

# Build the binary
go build -o sigil cmd/sigil/main.go

# Install to your PATH
sudo mv sigil /usr/local/bin/
```

## Configuration

Sigil uses a YAML configuration file located at `.sigil/config.yml` in your project root:

```yaml
# Model configuration
models:
  default: "gpt-4"
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"
      models:
        - "gpt-4"
        - "gpt-3.5-turbo"
    anthropic:
      api_key: "${ANTHROPIC_API_KEY}"
      models:
        - "claude-3-opus-20240229"
        - "claude-3-sonnet-20240229"
    ollama:
      endpoint: "http://localhost:11434"
      models:
        - "llama2"
        - "codellama"

# Memory settings
memory:
  enabled: true
  max_entries: 1000
  retention_days: 30

# Sandbox settings
sandbox:
  enabled: true
  timeout: 300s
  max_concurrent: 5
```

### Environment Variables

- `OPENAI_API_KEY` - OpenAI API key
- `ANTHROPIC_API_KEY` - Anthropic API key
- `SIGIL_CONFIG` - Path to config file (default: `.sigil/config.yml`)
- `SIGIL_LOG_LEVEL` - Log level (debug, info, warn, error)

## Commands

### ask - Ask questions about code

Ask questions about your codebase with AI assistance.

```bash
# Ask about a specific file
sigil ask --file main.go "What does this file do?"

# Ask about a directory
sigil ask --dir src/ "What is the architecture of this module?"

# Ask about recent changes
sigil ask --git HEAD~3..HEAD "What changed in the last 3 commits?"

# Include memory context
sigil ask --include-memory "How does the authentication work?"
```

### edit - AI-powered code transformation

Transform code with AI assistance and automatic validation.

```bash
# Edit a single file
sigil edit --file auth.go "Add password hashing with bcrypt"

# Edit multiple files
sigil edit --file server.go --file client.go "Add retry logic with exponential backoff"

# Multi-agent editing with validation
sigil edit --multi-agent --file api.go "Refactor to use dependency injection"

# Auto-commit changes
sigil edit --commit --file config.go "Add environment variable support"
```

### explain - Get code explanations

Get detailed explanations of code functionality.

```bash
# Explain a file
sigil explain --file algorithm.go

# Explain with specific question
sigil explain --file parser.go --query "How does the tokenizer work?"

# Detailed explanation
sigil explain --detailed --file engine.go

# Output as JSON
sigil explain --file utils.go --json
```

### summarize - Generate code summaries

Create summaries of code files or directories.

```bash
# Summarize a directory
sigil summarize --dir src/

# Brief summary
sigil summarize --brief --file module.go

# Focus on specific aspect
sigil summarize --dir api/ --focus "security"

# Output as markdown
sigil summarize --dir docs/ --out summary.md
```

### review - AI-powered code review

Perform comprehensive code reviews with AI assistance.

```bash
# Review files
sigil review --file handler.go --file middleware.go

# Security-focused review
sigil review --security-check --dir src/auth/

# Include all severity levels
sigil review --severity all --file api.go

# Auto-fix issues
sigil review --auto-fix --file validation.go

# Output as SARIF for CI integration
sigil review --format sarif --dir . --out review.sarif
```

### diff - Analyze code differences

Analyze Git diffs with AI insights.

```bash
# Analyze working directory changes
sigil diff

# Analyze staged changes
sigil diff --staged

# Compare branches
sigil diff --branch feature/auth

# Analyze specific commit
sigil diff --commit abc123

# Detailed analysis
sigil diff --detailed --staged
```

### doc - Generate documentation

Generate documentation from code with AI assistance.

```bash
# Document a directory
sigil doc --dir src/ --out docs/

# Use specific template
sigil doc --template api-docs --file server.go

# Include private members
sigil doc --include-private --dir internal/

# Markdown format
sigil doc --format markdown --dir pkg/ --out API.md
```

### memory - Manage context memory

Manage Sigil's context memory system.

```bash
# List memory entries
sigil memory list

# Search memory
sigil memory search "authentication"

# Show memory statistics
sigil memory stats

# Clean old entries
sigil memory clean --older-than 30d

# Export memory
sigil memory export --format json --out memory-backup.json
```

### sandbox - Manage validation sandboxes

Manage isolated environments for change validation.

```bash
# List sandboxes
sigil sandbox list

# Create new sandbox
sigil sandbox create my-feature

# Execute command in sandbox
sigil sandbox exec my-feature "go test ./..."

# Validate changes
sigil sandbox validate my-feature

# Clean up sandboxes
sigil sandbox clean --all
```

### multiagent (multi) - Multi-agent task execution

Execute complex tasks using multiple AI agents for validation.

```bash
# Multi-agent refactoring
sigil multi --task "Refactor database layer to use repository pattern" --file db/

# With specific agent configuration
sigil multi --lead-model gpt-4 --reviewer-model claude-3 --task "Add comprehensive error handling"

# Custom consensus threshold
sigil multi --consensus-threshold 0.8 --task "Optimize performance bottlenecks" --dir src/
```

## Common Options

Most commands support these common flags:

### Input Options
- `--file, -f` - Input file(s)
- `--dir, -d` - Input directory
- `--recursive, -r` - Process directories recursively
- `--lines` - Specific line range (e.g., "10-20")
- `--git` - Git revision range
- `--staged` - Use staged files
- `--stdin` - Read from stdin

### Output Options
- `--out, -o` - Output file
- `--format` - Output format (text, json, markdown, etc.)
- `--json` - Output as JSON
- `--patch` - Output as patch file
- `--in-place` - Modify files in place

### Model Options
- `--model, -m` - Model to use
- `--include-memory` - Include memory context
- `--memory-depth` - Number of memory entries to include

## Examples

### Code Refactoring with Validation
```bash
# Refactor code with multi-agent validation
sigil edit --multi-agent --file legacy.go \
  "Refactor this code to follow SOLID principles and add unit tests"
```

### Security Review Pipeline
```bash
# Comprehensive security review
sigil review --security-check --severity all --format sarif \
  --dir src/ --out security-report.sarif
```

### Documentation Generation
```bash
# Generate complete API documentation
sigil doc --dir pkg/ --template api-docs \
  --include-private --format markdown --out docs/API.md
```

### Git Integration Workflow
```bash
# Review changes before commit
sigil diff --staged | sigil review --stdin --auto-fix

# Generate commit message
sigil diff --staged | sigil summarize --stdin --brief
```

## Architecture

Sigil follows a modular architecture:

- **CLI Layer**: Command parsing and execution
- **Agent System**: Multi-agent orchestration for validation
- **Model Providers**: Pluggable LLM backends
- **Git Integration**: Safe operations with worktree isolation
- **Memory System**: Persistent context storage
- **Sandbox Execution**: Isolated validation environments

## Safety Features

1. **Git Required**: All operations require a Git repository
2. **Sandbox Validation**: Changes validated in isolated worktrees
3. **Multi-Agent Consensus**: Multiple AI agents validate changes
4. **No Auto-Apply**: Changes require explicit confirmation
5. **Rollback Support**: All operations are Git-reversible

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with Go and love for developer productivity
- Inspired by the need for safe, intelligent code transformation
- Thanks to all contributors and the open-source community