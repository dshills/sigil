# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

### Development
```bash
# Run linting
golangci-lint run

# Run tests (once implemented)
go test ./...
go test -v ./...                    # Verbose output
go test -run TestName ./...         # Run specific test

# Build the binary
go build -o sigil

# Run the application
./sigil [subcommand] [flags] [args]

# Format code recursively
gofmt -w .                          # Resolve formatting issues recursively
```

### Git Operations
This project requires all operations to be performed within a Git repository. The tool uses Git for version control and creates temporary worktrees for sandbox validation.

## Architecture Overview

Sigil is a command-line tool for intelligent, autonomous code transformation using LLMs. The architecture follows these key patterns:

### Core Design Principles
- **Interface-based Model Abstraction**: The `Model` interface allows pluggable LLM backends (OpenAI, Anthropic, Ollama, MCP)
- **Git-centric Operations**: All operations require a Git repository; uses worktrees for safe validation
- **Markdown-based Memory**: Context and history stored in `.sigil/memory/` as Markdown files
- **Minimal Dependencies**: Policy to use standard library where possible
- **Single Static Binary**: Distributed as a single executable for all platforms

### Expected Project Structure
```
sigil/
├── cmd/               # CLI entry points
│   └── sigil/        # Main executable
├── internal/         # Private application code
│   ├── cli/         # Command implementations
│   ├── model/       # LLM interface and implementations
│   ├── git/         # Git operations and utilities
│   ├── sandbox/     # Sandbox execution environment
│   ├── memory/      # Markdown-based context storage
│   └── validation/  # Rule enforcement and validation
├── pkg/             # Public libraries (if any)
└── tests/           # Integration tests
```

### Key Components

1. **Model Interface**: Abstract interface for LLM interactions supporting multiple backends
2. **Sandbox Execution**: Creates temporary Git worktrees for safe code validation
3. **Multi-Agent System**: Lead agent with multiple reviewer agents for validation
4. **Rule Enforcement**: YAML-based rules in `.sigil/rules.yml` for code standards
5. **Memory System**: Persistent context storage in `.sigil/memory/` directory

### Command Structure
Main commands: `ask`, `edit`, `explain`, `summarize`, `review`, `diff`, `doc`

Each command supports:
- File/directory selection patterns
- Streaming output
- Markdown formatting
- Git integration (staging, commits)

### Important Configuration

**Linting**: Comprehensive golangci-lint configuration in `.golangci.yml` with security, performance, and style checks enabled.

**Git Requirements**: All operations must be performed within a Git repository. The tool will refuse to run outside of Git repos.

**Environment Variables**:
- Model API keys (OPENAI_API_KEY, ANTHROPIC_API_KEY, etc.)
- Custom model endpoints
- Debug/verbose flags

## Development Guidelines

1. Follow the existing golangci-lint configuration strictly
2. Use interfaces for all major components to maintain testability
3. Keep external dependencies minimal
4. Ensure all file operations respect Git boundaries
5. Use the standard Go project layout
6. Write tests alongside implementation
7. Document public APIs with godoc comments