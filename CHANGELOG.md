# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Complete MCP Implementation**: Full Model Context Protocol support with:
  - JSON-RPC 2.0 compliant protocol handlers
  - Tool calling and function execution capabilities
  - Resource management (list, read, subscribe to resources)
  - Prompt template support for server-provided templates
  - Connection pooling with LRU management for efficiency
  - Health monitoring with automatic server restart
  - Comprehensive error handling and retry logic
  - 43KB+ of test coverage with mock servers and comprehensive test suites
- Initial release of Sigil - AI-powered code transformation CLI tool
- Core CLI commands:
  - `ask` - Ask questions about code with context-aware responses
  - `edit` - Transform code with AI assistance and validation
  - `explain` - Get detailed explanations of code functionality
  - `summarize` - Generate concise summaries of codebases
  - `review` - AI-powered code review with multiple focus areas
  - `diff` - Analyze code differences with intelligent insights
  - `doc` - Generate comprehensive documentation
  - `memory` - Manage persistent context across sessions
  - `sandbox` - Manage validation sandboxes for safe code execution
  - `multiagent` - Orchestrate multiple AI agents for complex tasks

- Multi-LLM provider support:
  - OpenAI (GPT-4, GPT-3.5)
  - Anthropic (Claude 3)
  - Ollama (local models)
  - MCP (Model Context Protocol)

- Advanced features:
  - Multi-agent orchestration with lead and reviewer agents
  - Sandboxed validation using Git worktrees
  - Markdown-based memory system for context persistence
  - Rule-based validation and enforcement
  - Multiple output formats (Markdown, JSON, HTML, XML, SARIF, YAML)
  - Git integration with auto-commit capabilities

- Infrastructure:
  - Comprehensive configuration system (YAML-based)
  - Structured error handling with context
  - Detailed logging with multiple levels
  - Modular architecture with clean interfaces

- Developer tools:
  - Example configurations (basic, advanced, multi-provider, security-focused)
  - Example usage scripts (code review, refactoring, documentation generation)
  - GitHub Actions CI/CD workflows
  - Docker support with multi-platform builds
  - Comprehensive test suite framework

- Documentation:
  - Detailed README with installation and usage instructions
  - Architecture overview and component documentation
  - Command examples and common workflows
  - Troubleshooting guide

### Fixed
- MCP CLI status display compatibility with new ServerStatus struct
- Code formatting and linting issues (gofmt, goconst, misspell)
- Type mismatches in CLI format switches
- Proper constant usage throughout codebase

### Security
- Sandboxed execution environment for code validation
- No secrets in code validation
- Support for security-focused code reviews
- Git repository requirement for all operations

### Performance
- Efficient streaming output for real-time feedback
- Parallel agent execution for faster results
- Configurable timeouts and resource limits
- Smart caching for memory operations

## [0.1.0] - TBD

Initial public release. See [Unreleased] section for full feature list.