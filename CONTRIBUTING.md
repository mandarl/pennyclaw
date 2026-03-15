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

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Write tests for new functionality
- Keep functions small and focused
- Add comments for exported types and functions

## Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` — New feature
- `fix:` — Bug fix
- `docs:` — Documentation changes
- `refactor:` — Code refactoring
- `test:` — Adding or updating tests
- `chore:` — Maintenance tasks

## Areas for Contribution

- **New LLM providers** — Add support for more providers
- **New skills** — Extend the agent's capabilities
- **New channels** — Telegram, Discord, Slack, etc.
- **Memory improvements** — Better context management
- **Documentation** — Guides, examples, translations
- **Testing** — Unit tests, integration tests
- **Performance** — Memory optimization for the free tier

## Reporting Issues

When reporting bugs, please include:
- PennyClaw version (`pennyclaw --version`)
- Go version (`go version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
