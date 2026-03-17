# PennyClaw Architecture

This document describes the internal architecture of PennyClaw, covering the major subsystems, their interactions, and the design decisions behind them.

## Overview

PennyClaw is a single-binary Go application that implements a personal AI agent. It follows a layered architecture with clear separation between communication channels, the agent core, intelligence (LLM + skills), storage, and infrastructure.

```
┌─────────────────────────────────────────────────────────┐
│                    Communication Layer                    │
│   Web UI (:3000)  │  Telegram Bot  │  Webhook Endpoint   │
├─────────────────────────────────────────────────────────┤
│                      Agent Core                          │
│   Message Handler  │  Context Builder  │  Tool Executor   │
├─────────────────────────────────────────────────────────┤
│                    Intelligence Layer                     │
│   LLM Gateway  │  Skills Registry  │  Skill Packs        │
├─────────────────────────────────────────────────────────┤
│                      Storage Layer                        │
│   SQLite (memory)  │  JSON (tasks)  │  Markdown (notes)   │
├─────────────────────────────────────────────────────────┤
│                    Infrastructure Layer                   │
│   Sandbox  │  Cron  │  Health  │  Logger  │  Config       │
└─────────────────────────────────────────────────────────┘
```

## Agent Loop

The core of PennyClaw is the agent loop in `internal/agent/agent.go`. When a message arrives from any channel, the agent:

1. **Saves** the user message to SQLite.
2. **Loads** conversation history (up to `max_history` messages).
3. **Builds** the system prompt by combining the base prompt with workspace context.
4. **Calls** the LLM with the full message history and available tools.
5. **Executes** any tool calls returned by the LLM, feeding results back as user messages.
6. **Repeats** steps 4-5 up to 10 times (the iteration cap prevents runaway loops).
7. **Returns** the final text response to the channel.

The agent tracks request latency, error counts, and tool call counts via the health checker. If the LLM provider doesn't support tool calling (detected at startup), the agent falls back to text-only mode.

## LLM Gateway

The LLM gateway (`internal/llm/provider.go`) abstracts away provider differences behind a unified interface:

```go
type Provider interface {
    Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error)
    SupportsTools() bool
}
```

Each provider (OpenAI, Anthropic, Gemini) implements this interface. The gateway handles:

- Request/response format translation
- API key authentication
- Token counting and limits
- Error handling and retries

The `openai-compatible` provider mode allows connecting to any API that follows the OpenAI chat completions format (OpenRouter, local models via Ollama, etc.).

## Skills System

Skills are the tools available to the agent. The registry (`internal/skills/skills.go`) manages skill registration, lookup, and execution.

Each skill is defined by:

- **Name**: Unique identifier (e.g., `task_add`)
- **Description**: Natural language description the LLM reads to decide when to use the skill
- **Parameters**: JSON Schema defining the expected arguments
- **Handler**: Go function that executes the skill

Skills are registered at startup in `agent.registerSkills()`. The registry converts skills to the LLM's tool format via `AsTools()`.

**Skill Packs** (`internal/skillpack/`) extend the system by loading skills from external YAML/JSON files. This allows adding new capabilities without modifying Go code.

## Communication Channels

### Web UI

The web server (`internal/channels/web/`) serves both the REST API and the embedded HTML/CSS/JS interface. The UI is compiled into the binary using Go's `embed` package (via the `html.go` file), so no external files are needed at runtime.

Key endpoints:

- `POST /api/chat` — Main chat endpoint (rate limited)
- `GET /api/health` — Health check with system metrics
- `GET /api/metrics` — Prometheus-compatible metrics
- `GET /api/config` — Configuration (sensitive fields redacted)

### Telegram Bot

The Telegram bot (`internal/channels/telegram/`) uses long polling (no webhook server needed). It communicates with the Telegram Bot API using raw `net/http` calls — no external Telegram library.

Features:

- Chat ID allowlist for access control
- Automatic message chunking (Telegram has a 4096-character limit)
- Markdown formatting with fallback to plain text on parse errors
- Graceful shutdown via context cancellation

### Webhook Endpoint

The webhook handler (`internal/channels/webhook/`) accepts HTTP POST requests from external services. It supports:

- HMAC-SHA256 signature verification (compatible with GitHub's `X-Hub-Signature-256`)
- Synchronous mode (waits for agent response)
- Asynchronous mode (returns 202 Accepted immediately)
- Plain text fallback when JSON parsing fails

## Storage

### SQLite (Conversation Memory)

The memory store (`internal/memory/store.go`) uses SQLite via `mattn/go-sqlite3`. It stores:

- **Sessions**: ID, name, creation time, last activity
- **Messages**: Session ID, role (user/assistant), content, channel, timestamp

The store handles session creation, message saving, history retrieval, and cleanup. It's the only component that requires CGO (for the SQLite C library).

### JSON (Task Store)

Tasks are stored in `data/tasks.json` as a simple JSON array. This was chosen over SQLite for several reasons:

- Tasks are a small dataset (typically < 100 items)
- JSON is human-readable and easy to backup
- No schema migrations needed
- Keeps the task system independent of the memory store

### Markdown (Knowledge Base)

Notes are stored as individual `.md` files in `data/notes/`. This makes them:

- Easy to browse and edit outside PennyClaw
- Compatible with any Markdown viewer
- Simple to backup (just copy the directory)

Note names are sanitized to prevent path traversal attacks.

## Infrastructure

### Sandbox

The sandbox (`internal/sandbox/sandbox.go`) provides isolation for command execution. On Linux, it uses:

- **Namespaces**: PID, mount, and network isolation
- **Cgroups**: Memory limits (configurable, default 128 MB)
- **Timeouts**: Configurable max execution time (default 30 seconds)
- **Working directory**: Restricted to the sandbox work dir

On non-Linux systems, the sandbox falls back to basic `exec.Command` with timeouts.

### Health Checker

The health checker (`internal/health/health.go`) collects:

- **System metrics**: Go version, goroutine count, memory stats (heap, stack, GC), disk usage, load average
- **Agent metrics**: Total requests, active requests, tool calls, average/P99 latency, error count
- **Health checks**: Memory usage thresholds, goroutine leak detection, disk space, error rate

Latency is tracked using a ring buffer of 1000 entries. P99 is calculated via insertion sort (O(n^2) but n is capped at 1000).

The Prometheus endpoint (`/api/metrics`) exports all metrics in text exposition format for scraping.

### Structured Logger

The logger (`internal/logging/logger.go`) supports:

- **Levels**: DEBUG, INFO, WARN, ERROR
- **Formats**: Human-readable (`2024-01-15T10:30:00Z [INFO] message`) or JSON lines
- **Components**: Child loggers with component prefixes (e.g., `[agent]`, `[telegram]`)
- **Caller info**: File and line number included for WARN and ERROR in structured mode
- **Key-value fields**: Arbitrary metadata attached to log entries

Child loggers share the parent's mutex to prevent interleaved writes to the same output.

### Config Validation

The config validator (`internal/config/validate.go`) runs at startup and checks:

- Port ranges and hostname format
- LLM provider validity and API key presence
- Unresolved environment variable references
- Memory and sandbox parameter ranges
- Channel-specific requirements (e.g., token required when Telegram is enabled)
- Email configuration completeness

Validation errors are collected and reported together, so operators can fix all issues in one pass.

## Design Decisions

**Why Go?** Go produces a single static binary with no runtime dependencies. The binary is ~16 MB and starts in milliseconds. This is critical for the e2-micro VM where every MB of RAM matters.

**Why zero external dependencies?** Each dependency adds binary size, potential security vulnerabilities, and maintenance burden. The Telegram bot API is simple enough to call with `net/http`. SMTP is in the standard library. HMAC-SHA256 is in `crypto/hmac`. The only exception is SQLite, which requires a C library.

**Why SQLite over PostgreSQL?** SQLite runs in-process with zero configuration. There's no separate database server consuming RAM on the e2-micro VM. For a single-user agent, SQLite's concurrency limitations are irrelevant.

**Why JSON for tasks instead of SQLite?** Tasks are a small, simple dataset. JSON keeps them human-readable and independent of the memory store. If the task count grows significantly, migration to SQLite would be straightforward.

**Why ring buffer for latency tracking?** A ring buffer has O(1) insertion and bounded memory usage. For 1000 float64 entries, it uses exactly 8 KB. This is important on a memory-constrained VM.
