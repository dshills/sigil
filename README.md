# Sigil

Sigil is an intelligent, autonomous code transformation CLI tool that leverages Large Language Models (LLMs) to understand, modify, and enhance codebases.

## Overview

Sigil provides a suite of AI-powered commands for code analysis, transformation, and documentation. It supports multiple LLM backends, implements sandboxed validation, and maintains context through a Markdown-based memory system.

## Features

- **Multi-LLM Support**: Works with OpenAI, Anthropic, Ollama, and MCP (Model Context Protocol) providers
- **Git-Centric Workflow**: All operations require a Git repository for safety and version control
- **Sandboxed Execution**: Validates changes in isolated Git worktrees before applying
- **Memory Persistence**: Maintains context across sessions using Markdown files
- **Multi-Agent System**: Uses lead and reviewer agents for comprehensive code validation
- **Flexible Output Formats**: Supports Markdown, JSON, HTML, XML, SARIF, and more

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/dshills/sigil.git
cd sigil

# Build the binary
make build

# Install to your PATH
make install
```

### Prerequisites

- Go 1.24.4 or higher
- Git (required for all operations)
- API keys for your chosen LLM provider(s)

## Configuration

Sigil uses a YAML configuration file located at `.sigil/config.yml` in your project root:

```yaml
models:
  lead: "openai:gpt-4"
  reviewers:
    - "anthropic:claude-3-sonnet"
  configs:
    openai:
      api_key: ${OPENAI_API_KEY}
    anthropic:
      api_key: ${ANTHROPIC_API_KEY}

rules:
  style:
    - identifier: "naming-convention"
      description: "Use camelCase for variables"
      severity: "warning"
  
memory:
  enabled: true
  max_entries: 100
```

### Environment Variables

- `OPENAI_API_KEY`: OpenAI API key
- `ANTHROPIC_API_KEY`: Anthropic API key
- `OLLAMA_HOST`: Ollama server URL (default: http://localhost:11434)

## Commands

### ask - Ask questions about code

```bash
# Ask a general question
sigil ask "How does the authentication system work?"

# Ask about specific files
sigil ask -f main.go "What does this file do?"

# Include memory context
sigil ask --include-memory "What changes were made to the API?"
```

### edit - Transform code with AI assistance

```bash
# Edit files with instructions
sigil edit main.go -d "Add comprehensive error handling"

# Auto-commit changes
sigil edit src/*.go -d "Add logging statements" --auto-commit

# Dry run to preview changes
sigil edit config.yaml -d "Add production settings" --dry-run
```

### explain - Get AI explanations of code

```bash
# Explain a single file
sigil explain main.go

# Explain with specific focus
sigil explain auth/*.go -q "How does token validation work?"

# Get detailed explanation
sigil explain database.go --detailed
```

### summarize - Generate code summaries

```bash
# Summarize a directory
sigil summarize src/

# Brief summary
sigil summarize *.go --brief

# Focus on specific aspect
sigil summarize internal/ --focus "security"
```

### review - AI-powered code review

```bash
# Review changes
sigil review main.go

# Security-focused review
sigil review --check-security src/

# Auto-fix issues
sigil review *.js --auto-fix

# Output as SARIF for CI integration
sigil review --format sarif -o review.sarif
```

### diff - Analyze code differences

```bash
# Analyze working directory changes
sigil diff

# Analyze staged changes
sigil diff --staged

# Compare with branch
sigil diff --branch main

# Get summary only
sigil diff --summary
```

### doc - Generate documentation

```bash
# Generate docs for files
sigil doc src/*.go -o docs/

# Include private members
sigil doc --include-private main.go

# Use specific template
sigil doc --template api-reference internal/
```

### memory - Manage context memory

```bash
# Show memory status
sigil memory show

# Search memory
sigil memory search "API changes"

# Clean old entries
sigil memory clean --before 30d
```

### sandbox - Manage validation sandboxes

```bash
# List active sandboxes
sigil sandbox list

# Validate changes in sandbox
sigil sandbox validate --file changes.patch

# Clean up sandboxes
sigil sandbox clean
```

## Common Workflows

### Code Review Workflow

```bash
# 1. Make changes to your code
vim src/feature.go

# 2. Review the changes
sigil review src/feature.go --check-security --check-performance

# 3. Apply suggested fixes
sigil review src/feature.go --auto-fix

# 4. Generate documentation
sigil doc src/feature.go -o docs/
```

### Refactoring Workflow

```bash
# 1. Analyze current code
sigil explain src/ -q "What are the main components?"

# 2. Plan refactoring
sigil ask "How can I improve the architecture of src/?"

# 3. Apply transformations
sigil edit src/*.go -d "Refactor to use dependency injection"

# 4. Review changes
sigil diff --detailed
```

### Documentation Workflow

```bash
# 1. Generate comprehensive docs
sigil doc --recursive src/ -o documentation/

# 2. Add code examples
sigil edit documentation/*.md -d "Add usage examples"

# 3. Create summary
sigil summarize documentation/ -o README_API.md
```

## Architecture

### Project Structure

```
sigil/
├── cmd/sigil/          # CLI entry point
├── internal/
│   ├── agent/          # Multi-agent orchestration
│   ├── cli/            # Command implementations
│   ├── config/         # Configuration management
│   ├── git/            # Git integration
│   ├── memory/         # Context persistence
│   ├── model/          # LLM interfaces
│   ├── sandbox/        # Validation environment
│   └── validation/     # Rule enforcement
└── spec/               # Formal specification
```

### Key Components

- **Model Interface**: Abstract interface supporting multiple LLM providers
- **Agent System**: Lead agent with reviewer agents for validation
- **Sandbox**: Temporary Git worktrees for safe validation
- **Memory**: Markdown-based context storage in `.sigil/memory/`

## Development

### Building from Source

```bash
# Run tests
make test

# Run linter
make lint

# Run all checks
make check

# Build for all platforms
make build-all
```

### Running Tests

```bash
# Unit tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/cli -v
```

## Troubleshooting

### Common Issues

1. **"not in a git repository" error**
   - Ensure you're running Sigil from within a Git repository
   - Initialize Git with `git init` if needed

2. **Model API errors**
   - Verify API keys are set correctly
   - Check network connectivity
   - Ensure model names are correct (e.g., "openai:gpt-4", "anthropic:claude-3-sonnet")

3. **Sandbox validation fails**
   - Check Git worktree support: `git worktree list`
   - Ensure sufficient disk space
   - Try `sigil sandbox clean` to remove stale sandboxes

### Debug Mode

Enable verbose logging:

```bash
sigil --verbose <command>
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices and idioms
- Add tests for new functionality
- Update documentation as needed
- Run `make check` before submitting PR
- Keep external dependencies minimal

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI structure
- Uses [go-git](https://github.com/go-git/go-git) for Git operations
- Inspired by AI-assisted development tools
- Developed with [Claude Code](https://claude.ai/code) - Anthropic's AI coding assistant
- Influenced by [OpenAI Codex](https://openai.com/blog/openai-codex) and modern AI pair programming tools

## Roadmap

- [ ] Support for more LLM providers
- [ ] Enhanced memory search capabilities
- [ ] Plugin system for custom commands
- [ ] Web UI for project visualization
- [ ] Integration with popular IDEs
- [ ] Distributed agent execution
- [ ] Custom rule definitions

## Support

For issues, questions, or contributions, please visit the [GitHub repository](https://github.com/dshills/sigil).