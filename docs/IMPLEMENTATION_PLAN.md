# Sigil Implementation Plan

## Overview
This document provides a detailed implementation plan for the Sigil CLI tool based on the formal specification. The implementation will be broken down into phases, with each phase building upon the previous one.

## Phase 1: Core Foundation (Week 1-2)

### 1.1 Project Structure Setup
- [x] Base directory structure (already in place)
- [ ] Enhance CLI command structure using cobra
- [ ] Setup comprehensive error handling framework
- [ ] Implement logging infrastructure

### 1.2 Model Interface & Abstraction
**Location**: `internal/model/`

- [ ] Define core `Model` interface
  ```go
  type Model interface {
      RunPrompt(ctx context.Context, input PromptInput) (PromptOutput, error)
  }
  ```
- [ ] Implement model registry pattern
- [ ] Create model configuration loader
- [ ] Define prompt input/output structures

### 1.3 Configuration System
**Location**: `internal/config/`

- [ ] YAML configuration parser
- [ ] Environment variable support
- [ ] Configuration validation
- [ ] Default configuration templates

### 1.4 Git Integration Foundation
**Location**: `internal/git/`

- [ ] Git repository detection
- [ ] Basic Git operations wrapper
- [ ] Worktree management
- [ ] Diff generation utilities

## Phase 2: CLI Commands Implementation (Week 3-4)

### 2.1 Base Command Structure
**Location**: `internal/cli/`

- [ ] Implement command interface
- [ ] Create shared flag parsing
- [ ] Input source handlers (file, dir, stdin, git)
- [ ] Output formatters (text, json, patch)

### 2.2 Core Commands
Each command in `internal/cli/commands/`

- [ ] `ask` command
  - Question parsing
  - Context gathering
  - Response formatting
- [ ] `edit` command
  - Instruction parsing
  - Code modification
  - Patch generation
- [ ] `explain` command
  - Code selection
  - Explanation generation
- [ ] `summarize` command
  - File/directory analysis
  - Summary generation
- [ ] `review` command
  - Code analysis
  - Review generation
- [ ] `diff` command
  - Diff parsing
  - Patch application
- [ ] `doc` command
  - Documentation generation
  - Comment insertion

## Phase 3: Model Implementations (Week 5-6)

### 3.1 OpenAI Provider
**Location**: `internal/model/providers/openai/`

- [ ] API client implementation
- [ ] Token management
- [ ] Streaming support
- [ ] Error handling

### 3.2 Anthropic Provider
**Location**: `internal/model/providers/anthropic/`

- [ ] API client implementation
- [ ] Message formatting
- [ ] Streaming support
- [ ] Error handling

### 3.3 Ollama Provider
**Location**: `internal/model/providers/ollama/`

- [ ] Local API client
- [ ] Model management
- [ ] Streaming support
- [ ] Error handling

### 3.4 MCP Provider
**Location**: `internal/model/providers/mcp/`

- [ ] HTTP client implementation
- [ ] Tool calling support
- [ ] Session management
- [ ] Async operations

## Phase 4: Memory System (Week 7)

### 4.1 Markdown Memory Storage
**Location**: `internal/memory/`

- [ ] Memory file structure
- [ ] Read/write operations
- [ ] Memory indexing
- [ ] Context retrieval

### 4.2 Memory Integration
- [ ] Global memory (`~/.sigil/memory/`)
- [ ] Local memory (`.sigil/memory/`)
- [ ] Task tracking (TODO.md, COMPLETED.md)
- [ ] Memory inclusion flags

## Phase 5: Sandbox & Validation (Week 8-9)

### 5.1 Sandbox Implementation
**Location**: `internal/sandbox/`

- [ ] Temporary workspace creation
- [ ] Git worktree management
- [ ] File overlay system
- [ ] Cleanup mechanisms

### 5.2 Validation Framework
**Location**: `internal/validation/`

- [ ] Rule definition structure
- [ ] Rule executor
- [ ] Linter integration
- [ ] Test runner integration
- [ ] Formatter integration

### 5.3 Autonomous Mode
- [ ] Auto-apply logic
- [ ] Validation checks
- [ ] Rollback mechanisms
- [ ] Commit automation

## Phase 6: Multi-Agent System (Week 10-11)

### 6.1 Agent Orchestration
**Location**: `internal/agent/`

- [ ] Lead agent implementation
- [ ] Reviewer agent framework
- [ ] Agent communication protocol
- [ ] Consensus mechanisms

### 6.2 Review System
- [ ] Review comment generation
- [ ] Patch suggestions
- [ ] Voting system
- [ ] Conflict resolution

## Phase 7: Advanced Features (Week 12)

### 7.1 Rule Enforcement
**Location**: `internal/rules/`

- [ ] Rule configuration parser
- [ ] Rule executor
- [ ] Pre-commit hooks
- [ ] CI/CD integration

### 7.2 Advanced Git Operations
- [ ] Checkpoint commits
- [ ] Branch management
- [ ] Merge conflict resolution
- [ ] PR preparation

## Phase 8: Testing & Documentation (Week 13-14)

### 8.1 Testing Suite
**Location**: `tests/`

- [ ] Unit tests for all packages
- [ ] Integration tests
- [ ] End-to-end tests
- [ ] Fuzz tests for prompt formatting
- [ ] Mock implementations

### 8.2 Documentation
- [ ] API documentation
- [ ] User guide
- [ ] Configuration reference
- [ ] Example workflows

## Phase 9: Build & Release (Week 15)

### 9.1 Build System
- [ ] Cross-compilation setup
- [ ] Static binary generation
- [ ] Version management
- [ ] Release automation

### 9.2 Distribution
- [ ] Binary packaging
- [ ] Installation scripts
- [ ] Homebrew formula
- [ ] Docker image

## Implementation Priority Order

1. **Core Infrastructure** (Must have first)
   - Model interface
   - Basic CLI structure
   - Git integration
   - Configuration system

2. **Essential Commands** (MVP)
   - `ask`, `edit`, `explain`
   - OpenAI provider
   - Basic validation

3. **Enhanced Features**
   - All remaining commands
   - Additional providers
   - Memory system
   - Sandbox execution

4. **Advanced Features**
   - Multi-agent system
   - Rule enforcement
   - Autonomous mode

## Technical Decisions

### Dependencies
- **CLI Framework**: cobra (for robust command handling)
- **HTTP Client**: Standard library with context support
- **YAML Parsing**: gopkg.in/yaml.v3
- **Testing**: Standard library + testify for assertions
- **Logging**: slog (structured logging)

### Architecture Patterns
- **Dependency Injection**: For testability
- **Interface-based design**: For flexibility
- **Context propagation**: For cancellation and timeouts
- **Error wrapping**: For better debugging

### Code Organization
- **Packages**: Small, focused, single-responsibility
- **Interfaces**: Defined in consumer packages
- **Testing**: Parallel to implementation
- **Documentation**: Inline with code

## Risk Mitigation

1. **LLM API Changes**: Abstract behind interfaces
2. **Git Operations**: Comprehensive error handling
3. **File System**: Strict permission checks
4. **Memory Growth**: Size limits and rotation
5. **Concurrent Access**: Proper locking mechanisms

## Success Metrics

- All commands functional with at least one provider
- 80%+ test coverage
- Sub-second response time for local operations
- Zero data loss scenarios
- Cross-platform compatibility

## Next Steps

1. Begin with Phase 1 core foundation
2. Implement model interface and basic CLI
3. Add OpenAI provider for initial testing
4. Iterate based on early feedback