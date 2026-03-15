# 🪙 PennyClaw

**Your $0/month personal AI agent, running 24/7 on GCP's free tier.**

[![Deploy to GCP](https://gstatic.com/cloudssh/images/open-btn.svg)](https://shell.cloud.google.com/cloudshell/editor?cloudshell_git_repo=https://github.com/mandarl/pennyclaw.git&cloudshell_tutorial=docs/deploy-tutorial.md&cloudshell_workspace=.)
[![CI](https://github.com/mandarl/pennyclaw/actions/workflows/ci.yml/badge.svg)](https://github.com/mandarl/pennyclaw/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)

---

PennyClaw is a lightweight, open-source AI agent built from scratch in Go, designed to run comfortably within the constraints of Google Cloud Platform's **Always Free** `e2-micro` VM (1GB RAM, 2 shared vCPUs, 30GB disk). One click deploys it. Zero dollars keeps it running. Forever.

## Why PennyClaw?

| | OpenClaw | NanoClaw | PennyClaw |
|---|---|---|---|
| **RAM Usage** | 2-4 GB | 200-500 MB | **< 50 MB idle** |
| **Monthly Cost** | $5-20/mo VPS | $5-20/mo VPS | **$0/mo** |
| **Deployment** | Complex setup | Docker required | **One click** |
| **Language** | TypeScript | TypeScript | **Go** |
| **Codebase** | 500k+ lines | ~500 lines | **~2,000 lines** |

> *"I was tired of paying for servers I barely use. GCP gives everyone a free VM forever. So I built PennyClaw."*

## Demo

<p align="center">
  <img src="https://d2xsxph8kpxj0f.cloudfront.net/310519663317149291/VWpALcgzAw8ss345SKvWHs/demo_7bac11e6.svg" alt="PennyClaw deploy demo" width="720">
</p>

## Quick Start

### Option 1: One-Click Deploy to GCP (Recommended)

Click the button below to deploy PennyClaw to your own GCP free-tier VM in under 5 minutes:

[![Open in Cloud Shell](https://gstatic.com/cloudssh/images/open-btn.svg)](https://shell.cloud.google.com/cloudshell/editor?cloudshell_git_repo=https://github.com/mandarl/pennyclaw.git&cloudshell_tutorial=docs/deploy-tutorial.md&cloudshell_workspace=.)

The deployment script includes **24 pre-flight checks** to ensure you stay within the free tier:

- ✅ Detects existing e2-micro instances (only 1 is free)
- ✅ Validates region eligibility (us-west1, us-central1, us-east1)
- ✅ Guards against premium network tier charges
- ✅ Verifies disk type and size limits
- ✅ Shows a $0.00 cost breakdown before deploying
- ✅ Auto-configures swap for ~1.5GB effective RAM
- ✅ Generates a one-command teardown script

### Option 2: Run Locally

```bash
git clone https://github.com/mandarl/pennyclaw.git
cd pennyclaw
cp config.example.json config.json

# Set your API key
export OPENAI_API_KEY="sk-your-key-here"

# Build and run
make run
```

Open http://localhost:3000 in your browser.

### Option 3: Docker

```bash
git clone https://github.com/mandarl/pennyclaw.git
cd pennyclaw
docker build -t pennyclaw .
docker run -p 3000:3000 \
  -e OPENAI_API_KEY="sk-your-key-here" \
  pennyclaw
```

## Features

### Core
- **Multi-provider LLM gateway** — OpenAI, Anthropic, Google Gemini, OpenRouter, and any OpenAI-compatible API
- **Persistent memory** — SQLite-backed conversation history that survives restarts
- **Tool execution** — Sandboxed shell commands, file I/O, web search, HTTP requests
- **Web chat UI** — Clean, embedded interface with zero external dependencies

### Deployment
- **One-click GCP deploy** — Guided Cloud Shell tutorial with automated setup
- **24 pre-flight checks** — Validates free tier eligibility before spending a cent
- **Auto-swap config** — 512MB swap file extends effective RAM to ~1.5GB
- **systemd service** — Auto-restarts on crash, starts on boot
- **Unattended upgrades** — Automatic security patches

### Security
- **Native Linux sandboxing** — Namespaces and cgroups, no Docker daemon overhead
- **Non-root execution** — Runs as dedicated `pennyclaw` user
- **systemd hardening** — `ProtectSystem=strict`, `NoNewPrivileges`, `PrivateTmp`
- **Memory limits** — Cgroup-enforced 800MB ceiling prevents OOM kills

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    PennyClaw                         │
│                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │ Web UI   │  │ Telegram │  │ Discord          │  │
│  │ :3000    │  │ Bot      │  │ Bot              │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────────────┘  │
│       │              │              │                │
│       └──────────────┼──────────────┘                │
│                      │                               │
│              ┌───────▼────────┐                      │
│              │   Agent Loop   │                      │
│              │  (core logic)  │                      │
│              └───┬───────┬───┘                      │
│                  │       │                           │
│          ┌───────▼──┐ ┌──▼────────┐                 │
│          │ LLM      │ │ Skills    │                 │
│          │ Provider  │ │ Registry  │                 │
│          └──────────┘ └──┬────────┘                 │
│                          │                           │
│              ┌───────────▼──────────┐               │
│              │   Sandbox            │               │
│              │ (namespaces/cgroups) │               │
│              └──────────────────────┘               │
│                                                     │
│  ┌──────────────┐                                   │
│  │ SQLite Memory │  < 50MB idle / < 200MB active    │
│  └──────────────┘                                   │
└─────────────────────────────────────────────────────┘
```

## GCP Free Tier Specs

PennyClaw is architected for these exact constraints:

| Resource | Free Tier | PennyClaw Usage |
|---|---|---|
| VM | 1x e2-micro/month | 1x e2-micro |
| vCPU | 2 shared cores | ~5% idle |
| RAM | 1 GB | < 50 MB idle, < 200 MB active |
| Disk | 30 GB pd-standard | 30 GB |
| Egress | 1 GB/month (Americas) | ~50 MB/month typical |
| Regions | us-west1, us-central1, us-east1 | Auto-selected |

## Configuration

PennyClaw uses a single `config.json` file:

```json
{
  "llm": {
    "provider": "openai",
    "model": "gpt-4.1-mini",
    "api_key": "$OPENAI_API_KEY"
  },
  "channels": {
    "web": { "enabled": true },
    "telegram": { "enabled": false, "token": "$TELEGRAM_BOT_TOKEN" }
  }
}
```

Environment variables prefixed with `$` are automatically resolved.

### OpenRouter / Custom Providers

PennyClaw works with any OpenAI-compatible API. To use OpenRouter:

```json
{
  "llm": {
    "provider": "openai",
    "model": "anthropic/claude-sonnet-4-20250514",
    "api_key": "$OPENROUTER_API_KEY",
    "base_url": "https://openrouter.ai/api/v1"
  }
}
```

## Security

PennyClaw includes basic security features:

- **Authentication:** Set `PENNYCLAW_AUTH_TOKEN` env var to require a token for web UI access
- **Rate limiting:** 20 requests per minute per IP on the chat endpoint
- **Sandbox isolation:** Tool execution runs in a restricted environment
- **systemd hardening:** `ProtectSystem=strict`, `NoNewPrivileges=true`, memory limits

```bash
# Set auth token (strongly recommended for public-facing deployments)
export PENNYCLAW_AUTH_TOKEN="your-secret-token-here"
```

Without `PENNYCLAW_AUTH_TOKEN`, the web UI is open to anyone who discovers your IP.

## Built-in Skills

| Skill | Description |
|---|---|
| `run_command` | Execute sandboxed shell commands |
| `read_file` | Read file contents |
| `write_file` | Create or overwrite files |
| `web_search` | Search the web via DuckDuckGo |
| `http_request` | Make HTTP requests to APIs |

## Pre-Flight Checks

Run `make preflight` to validate your GCP setup without deploying:

```
━━━ PHASE 1: GCP Account & Authentication ━━━
  ✓  gcloud CLI installed (462.0.1)
  ✓  Authenticated as: user@gmail.com
  ✓  Project: my-project-123
  ✓  Billing is enabled

━━━ PHASE 2: Free Tier Eligibility ━━━
  ✓  No existing e2-micro instances — you're eligible!
  ✓  No existing disks — full 30GB available

━━━ PHASE 3: Region Selection ━━━
  ✓  Selected: us-central1 (42ms latency)

━━━ PHASE 4: Cost Protection ━━━
  ✓  Machine type: e2-micro
  ✓  Disk: 30GB pd-standard
  ✓  Network: STANDARD tier

━━━ PHASE 5: Cost Summary ━━━
  TOTAL: $0.00/month ✓
```

## Teardown

Remove everything with one command:

```bash
make teardown
```

This deletes the VM and firewall rules. No further charges.

## Contributing

PennyClaw is MIT licensed. Contributions welcome!

1. Fork the repo
2. Create a feature branch
3. Make your changes
4. Run `make test`
5. Submit a PR

## License

MIT License. See [LICENSE](LICENSE) for details.

---

**PennyClaw** — Because your AI agent shouldn't cost more than a penny.
