# Contributing to PennyClaw

Thank you for your interest in contributing to PennyClaw! This document provides guidelines for contributing to the project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/pennyclaw.git`
3. Create a branch: `git checkout -b feature/your-feature`
4. Make your changes
5. Run tests: `make test`
6. Commit: `git commit -m "feat: your feature description"`
7. Push: `git push origin feature/your-feature`
8. Open a Pull Request

## Development Setup

**Prerequisites:**

- Go 1.22+
- GCC (for CGO/SQLite)
- Make

```bash
cp config.example.json config.json
# Edit config.json with your API key
make run
```

The web UI will be available at http://localhost:3000.

## Project Structure

```
cmd/pennyclaw/          Main entry point, CLI flags, signal handling
internal/
  agent/                Agent loop: context building, LLM calls, tool execution
  channels/
    web/                HTTP server, web UI (embedded HTML), REST API
    telegram/           Telegram bot (long polling, no external deps)
    webhook/            Webhook endpoint with HMAC-SHA256 verification
  config/               Config loading, env var resolution, validation
  cron/                 Cron scheduler for recurring tasks
  health/               Health checks, system metrics, Prometheus endpoint
  llm/                  Multi-provider LLM gateway (OpenAI, Anthropic, Gemini)
  logging/              Structured leveled logger (JSON or human-readable)
  memory/               SQLite-backed conversation store
  notify/               Email notifications via SMTP
  sandbox/              Linux namespace/cgroup sandboxing for tool execution
  skillpack/            External skill loader (YAML/JSON bundles)
  skills/               Skill registry, built-in skills, productivity tools
  workspace/            Persistent file workspace with bootstrap support
scripts/                Deployment and teardown scripts
docs/                   Deploy tutorial, assets
```

## Design Principles

PennyClaw follows several design principles that contributors should keep in mind:

**Zero external dependencies for runtime features.** Every feature is implemented using the Go standard library. The Telegram bot uses `net/http` directly, email uses `net/smtp`, webhook verification uses `crypto/hmac`. The only external dependency is `mattn/go-sqlite3` for persistent storage. This keeps the binary small (~16 MB) and the attack surface minimal.

**Fit within GCP e2-micro constraints.** Every feature must work within 1 GB RAM and 2 shared vCPUs. Use atomic operations and ring buffers instead of unbounded data structures. Prefer JSON file storage over additional databases. Profile memory usage when adding new features.

**Startup validation over runtime surprises.** The config validator catches common mistakes (missing API keys, invalid ports, unresolved env vars) at startup with actionable error messages. New config fields should include validation rules.

## Adding a New Skill

Skills are registered in the `internal/skills/` package. To add a new skill:

1. Define the skill in a new file (e.g., `internal/skills/my_skill.go`)
2. Create a registration function: `func RegisterMySkills(r *Registry, ...)`
3. Wire it into `internal/agent/agent.go` in the `registerSkills()` method
4. Add tests in `internal/skills/my_skill_test.go`

Each skill needs:

- A unique `Name` (snake_case, e.g., `my_action`)
- A `Description` that the LLM reads to decide when to use the skill
- `Parameters` as a JSON Schema object
- A `Handler` function that receives `context.Context` and `json.RawMessage`

## Adding a New Channel

Channels live in `internal/channels/`. Each channel:

1. Receives messages from an external source
2. Calls the agent's `HandleMessage(ctx, sessionID, message, channel)` function
3. Returns the response to the user

See `internal/channels/telegram/telegram.go` for a complete example.

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Write tests for new functionality
- Keep functions small and focused
- Add comments for exported types and functions
- Use `context.Context` for cancellation and timeouts
- Prefer returning errors over panicking

## Testing

Run the full test suite:

```bash
make test          # All tests with race detector
go test ./...      # Quick run without race detector
go test -v ./internal/skills/  # Single package
```

The project currently has 110+ tests across 12 packages. New features should include tests.

## Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` — New feature
- `fix:` — Bug fix
- `docs:` — Documentation changes
- `refactor:` — Code refactoring
- `test:` — Adding or updating tests
- `chore:` — Maintenance tasks

## Areas for Contribution

- **New LLM providers** — Add support for more providers in `internal/llm/`
- **New skills** — Extend the agent's capabilities (calendar, RSS, etc.)
- **Discord bot** — Implement the Discord channel (config exists, bot does not)
- **Streaming responses** — Add SSE/WebSocket streaming for the web UI
- **Memory improvements** — Semantic search, summarization, context compression
- **Documentation** — Guides, examples, translations
- **Performance** — Memory optimization for the free tier
- **UI improvements** — Better mobile experience, accessibility

## Reporting Issues

When reporting bugs, please include:

- PennyClaw version (`pennyclaw --version`)
- Go version (`go version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior
- Relevant log output (from `/api/logs` or `journalctl -u pennyclaw`)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
