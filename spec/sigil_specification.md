# Sigil CLI Tool – Formal Specification

## 1. Overview

**Sigil** is a command-line tool written in Go that enables intelligent, autonomous code transformation, explanation, review, and generation using local or remote LLMs. It is inspired by tools like Codex and Claude-Code, but emphasizes automation, multi-agent collaboration, and Git-integrated workflows.

Sigil supports interaction with multiple LLMs, sandboxed validation, fully autonomous execution, memory persistence via Markdown files, and integration with MCP servers for orchestration.

---

## 2. Functional Requirements

### 2.1 CLI Commands

#### Base Syntax
~~~sh
sigil [subcommand] [flags] [args]
~~~

#### Subcommands
- `ask` – Ask a question about code
- `edit` – Modify code using an instruction
- `explain` – Explain selected code
- `summarize` – Provide high-level overview of file or project
- `review` – AI-based code review
- `diff` – Show and optionally apply a patch
- `doc` – Generate documentation/comments

### 2.2 Input Sources
- File (`--file`)
- Directory (`--dir`)
- Selected line range (`--lines`)
- Git diff (`--git`, `--staged`)
- Standard input (stdin)

### 2.3 Output Modes
- Plain text (default)
- JSON (`--json`)
- Patch mode (`--patch`)
- Replace in-place (`--in-place`)
- Write to file (`--out <file>`)

### 2.4 Git Integration
- Sigil must be executed within a Git repository
- Uses Git to:
  - Track diffs and working state
  - Create checkpoint commits
  - Read staged changes

---

## 3. Model Backends

### 3.1 Model Abstraction

~~~go
type Model interface {
    RunPrompt(ctx context.Context, input PromptInput) (PromptOutput, error)
}
~~~

### 3.2 Supported Providers
- OpenAI (API Key)
- Anthropic Claude
- Ollama (local models)
- MCP servers (via HTTP API)

### 3.3 MCP Integration
- Support `callTool`, `callToolAsync`, and session memory
- Configurable via YAML:

~~~yaml
backend: mcp
mcp:
  server_url: http://localhost:8080
  model: sigil-agent
~~~

---

## 4. Automation & Sandbox

### 4.1 Autonomous Mode
- Flag: `--auto`
- No human confirmation required
- Self-apply changes, validate, and commit if successful

### 4.2 Sandbox Execution
- Use temp directories, `git worktree`, or overlays
- Validate code with:
  - Linters
  - Formatters
  - Test runners
- Revert if validation fails

---

## 5. Multi-Agent Orchestration

### 5.1 Primary + Reviewer Models

~~~yaml
models:
  lead: openai:gpt-4
  reviewers:
    - anthropic:claude-3-sonnet
    - local:codellama-70b
~~~

- Lead performs changes
- Reviewers comment, vote, or generate patches

---

## 6. Rule Enforcement

### 6.1 Configurable Rules

~~~yaml
rules:
  - must_pass: go test ./...
  - must_pass: golangci-lint run
  - must_pass: go vet ./...
~~~

- Exit code != 0 = fail
- Required before commit

---

## 7. Markdown-Based Memory System

### 7.1 Global & Local `SIGIL.md`
- Global: `~/.sigil/SIGIL.md`
- Local: `<repo>/.sigil/SIGIL.md`
- Contains contextual memory entries, model output summaries, decision history

### 7.2 Task Tracking
- `TODO.md`: Tasks planned or created by models/users
- `COMPLETED.md`: Completed tasks with metadata (timestamp, commit hash, model)

### 7.3 Model Access to Memory
- Controlled via flags:
  - `--include-memory`
  - `--memory-depth N`

---

## 8. Technical Constraints

### 8.1 Language
- Written in **Go (Golang)**
- Target version: Go 1.21+

### 8.2 Dependency Policy
- Minimal dependencies preferred
- Allowed with justification:
  - `google/uuid`
  - `database/sql` drivers
  - `yaml.v3` (if needed)
  - CLI helpers (`cobra`, optional)

### 8.3 Build Targets
- Single static binary
- Targets:
  - Linux (x64)
  - macOS (ARM64 & Intel)
  - Windows (x64)

---

## 9. Testing Requirements

- Unit tests (standard `testing` package)
- Integration tests for full flows
- Mocking allowed via `testify/mock` only when needed
- Fuzz tests for prompt formatting
- Snapshot test optional for response diffing

---

---

## 10. Future Enhancements (Planned)

- Vector-based semantic memory store
- PR generation
- Risk-based reviewer assignment
- Code coverage diff validation
- Chat/REPL interface for human override
