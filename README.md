[🇬🇧 English](#) | [🇷🇺 Русский](README.ru.md)

# uBot

```
                 ____        __
    __  ______  / __ )____  / /_
   / / / / __ \/ __  / __ \/ __/
  / /_/ / /_/ / /_/ / /_/ / /_
  \__,_/_.___/_____/\____/\__/

  The World's Most Lightweight
     Self-Hosted AI Assistant
```

**uBot** is the world's most lightweight self-hosted AI assistant. A complete rewrite of [nanobot](https://github.com/HKUDS/nanobot) in Go for maximum performance and security.

[![GitHub](https://img.shields.io/badge/GitHub-lubluniky%2Fubot-blue?logo=github)](https://github.com/lubluniky/ubot)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

## Features

- **Ultra-Lightweight** — ~12,000 lines of Go code (vs 400k+ in comparable projects)
- **Self-Hosted** — your data stays on your own hardware
- **Multi-Provider** — OpenRouter, GitHub Copilot, Anthropic, OpenAI, Ollama
- **Multi-Channel** — Telegram, WhatsApp (coming soon), CLI
- **Tool System** — files, shell, web search, web fetch, browser automation
- **Voice Support** — voice message transcription via Whisper (Groq/OpenAI)
- **Browser Automation** — headless Chrome via CDP with session persistence, anti-detection stealth, UA rotation, and proxy support
- **Proactive Cron** — the bot proactively sends messages on a schedule (reminders, monitoring)
- **Security Middleware** — protection against access to sensitive files and dangerous commands
- **Skill System** — 9 built-in skills + CLI management + SKILL.md extensions
- **Self-Management** — the bot can manage itself (config, restart) from CLI
- **MCP Support** — connect external tools via Model Context Protocol
- **Secure Sandbox** — Docker-based isolation with gVisor support
- **Interactive TUI** — interactive setup wizard

## Quick Start

### One-Line Install

```bash
curl -fsSL https://raw.githubusercontent.com/lubluniky/ubot/main/install.sh | bash
```

The installer will:
- Check your OS and dependencies
- Install Docker if needed
- Build the Docker image
- Launch the interactive setup wizard
- Create the `ubot` command

### Manual Install

```bash
git clone https://github.com/lubluniky/ubot.git
cd ubot
go build -o ubot ./cmd/ubot/
./ubot setup
```

## Usage

```bash
# Gateway (channels)
ubot start                    # Start the gateway (Telegram, etc.)
ubot stop                     # Stop the gateway
ubot restart                  # Restart the gateway
ubot logs                     # Show gateway logs

# Chat
ubot chat                     # Interactive chat mode
ubot chat -m "Hello!"         # Send a single message

# Configuration
ubot setup                    # Interactive setup wizard
ubot config                   # Open config file in editor
ubot status                   # Show current configuration
ubot version                  # Show version

# Skills Management
ubot skills list              # List installed and available skills
ubot skills install <name>    # Install a skill from the repository
ubot skills uninstall <name>  # Remove an installed skill
ubot skills info <name>       # Show skill details

# Self-Configuration
ubot rootchat                 # AI assistant for configuring uBot itself

# Maintenance
ubot update                   # Update to the latest version
ubot destroy                  # Complete removal
```

## Configuration

Config file: `~/.ubot/config.json`

```json
{
  "agents": {
    "defaults": {
      "model": "anthropic/claude-sonnet-4-20250514",
      "maxTokens": 4096,
      "temperature": 0.7
    }
  },
  "providers": {
    "openrouter": { "apiKey": "sk-or-v1-xxx" },
    "copilot": { "enabled": true, "accessToken": "gho_xxx" },
    "anthropic": { "apiKey": "sk-ant-xxx" },
    "openai": { "apiKey": "sk-xxx" },
    "ollama": { "apiBase": "http://localhost:11434/v1" }
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "123456:ABC...",
      "allowFrom": ["your_user_id"]
    }
  },
  "tools": {
    "web": {
      "search": { "apiKey": "BSA..." }
    },
    "browser": {
      "stealth": true,
      "proxy": "",
      "idleTimeout": 300
    }
  },
  "mcp": {
    "servers": []
  }
}
```

## Providers

| Provider | Description | API Key |
|----------|-------------|---------|
| **OpenRouter** | Access to Claude, GPT-4, Llama | [openrouter.ai/keys](https://openrouter.ai/keys) |
| **GitHub Copilot** | Free with GitHub subscription | Device Flow in setup |
| **Anthropic** | Claude directly | [console.anthropic.com](https://console.anthropic.com) |
| **OpenAI** | GPT-4 directly | [platform.openai.com](https://platform.openai.com) |
| **Ollama** | Local models | Not required |

## Skills

Skills extend the bot's capabilities. Create `~/.ubot/workspace/skills/{name}/SKILL.md`:

```markdown
# Code Review

Helps review code for bugs and improvements.

<!-- always-load -->

## Capabilities

- Bug detection
- Security checks
- Improvement suggestions

## Tools

- `read_file`: read files for analysis
- `exec`: run linters
```

The bot automatically discovers and suggests using relevant skills.

**Built-in skills:** code-review, web-research, data-analysis, writing-assistant, task-management, feature-spec, research-synthesis, sysadmin, meeting-notes.

## Voice (Whisper)

Voice messages in Telegram are automatically transcribed via the Whisper API:

- **Groq** (default if key is available) — `whisper-large-v3`
- **OpenAI** — `whisper-1`

Configuration in `config.json`:
```json
{
  "tools": {
    "voice": {
      "backend": "groq",
      "model": "whisper-large-v3"
    }
  }
}
```

Transcribed text is processed as a regular message.

## Browser Automation

The bot can control headless Chrome for web tasks:

```
"Go to example.com and tell me what's on the page"
"Log into my account on site.com" (use session: "mysite" to persist login)
"Find the Login button on the site and click it"
"Take a screenshot of the page"
"List my saved browser sessions"
```

Available actions: `browse_page`, `click_element`, `type_text`, `extract_text`, `screenshot`, `list_sessions`, `delete_session`.

### Session Persistence

Use the `session` parameter to keep cookies/logins across restarts. Named sessions are stored in `~/.ubot/workspace/browser-sessions/<name>/`. Without `session`, a temporary profile is used (wiped on close).

### Anti-Detection Stealth

When `stealth: true` (default), the browser:
- Hides `navigator.webdriver` fingerprint
- Mocks `navigator.plugins`, `chrome.runtime`, `chrome.app`, `chrome.csi`
- Overrides `navigator.permissions.query`
- Rotates User-Agent from a pool of 5 common desktop UAs
- Randomizes viewport size with small offsets
- Sets `--disable-blink-features=AutomationControlled` and other flags

### Proxy Support

Set a proxy in `config.json` — supports HTTP, HTTPS, SOCKS5:
```json
{ "tools": { "browser": { "proxy": "socks5://127.0.0.1:1080" } } }
```

### Browser Config

```json
{
  "tools": {
    "browser": {
      "sessionDir": "~/.ubot/workspace/browser-sessions",
      "proxy": "",
      "stealth": true,
      "idleTimeout": 300
    }
  }
}
```

The browser launches lazily on first use and shuts down after idle timeout (default: 5 minutes).

See [docs/linux-deploy.md](docs/linux-deploy.md) for Linux/Docker deployment with Chromium.

## Proactive Cron

The bot can proactively send messages on a schedule:

```
"Remind me to drink water every hour"
"Every day at 9:00 send me a weather summary"
```

The LLM manages the scheduler via the `cron` tool:
- `add` — add a job (cron expression or `@every 5m`)
- `remove` — remove a job
- `list` — show active jobs

Jobs are persisted in `~/.ubot/cron_jobs.json` and survive restarts.

## MCP (Model Context Protocol)

Connect external tools via MCP:

```json
{
  "mcp": {
    "servers": [
      {
        "name": "filesystem",
        "command": "npx",
        "args": ["-y", "@anthropic/mcp-server-filesystem", "/home/user"],
        "transport": "stdio"
      },
      {
        "name": "database",
        "url": "http://localhost:8080",
        "transport": "http"
      }
    ]
  }
}
```

MCP tools appear as `mcp_{server}_{tool}` in the available tools list.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      CHANNELS LAYER                          │
│             (Telegram, WhatsApp, CLI)                        │
└──────────────┬──────────────────────────────────────────────┘
               │
               ▼
      ┌────────────────────┐
      │    Message Bus     │
      └────────┬───────────┘
               │
               ▼
┌──────────────────────────────────────────────────────────────┐
│                    AGENT LOOP                                │
│  ├─ Context Builder (system prompt, skills)                 │
│  ├─ LLM Provider (OpenRouter, Copilot, etc.)               │
│  ├─ Tool Registry (file, shell, web, mcp)                  │
│  ├─ Skill Loader (SKILL.md files)                          │
│  ├─ MCP Manager (external tools)                           │
│  └─ Session Manager (conversation history)                  │
└──────────────────────────────────────────────────────────────┘
```

## Project Structure

```
ubot/
├── cmd/ubot/           # CLI entry point
│   └── cmd/            # Cobra commands
├── internal/
│   ├── agent/          # Agent loop, context, memory
│   ├── bus/            # Message bus
│   ├── channels/       # Telegram, WhatsApp
│   ├── config/         # Configuration
│   ├── cron/           # Proactive cron scheduler
│   ├── mcp/            # MCP client & manager
│   ├── providers/      # LLM providers
│   ├── sandbox/        # Docker sandboxing
│   ├── session/        # Conversation sessions
│   ├── skills/         # Skill loader, parser & manager
│   ├── tools/          # Built-in tools (security, browser, cron, manage)
│   ├── tui/            # Terminal UI
│   └── voice/          # Whisper transcription
├── skills/             # Bundled skills
├── docs/               # Deployment guides
├── install.sh          # One-line installer
├── Dockerfile
└── docker-compose.yml
```

## Docker

```bash
# Build
docker build -t ubot .

# Run gateway
docker run -d --name ubot \
  -v ~/.ubot:/home/ubot/.ubot \
  --security-opt no-new-privileges:true \
  --read-only \
  ubot gateway

# Interactive chat
docker run -it --rm \
  -v ~/.ubot:/home/ubot/.ubot \
  ubot agent
```

## Security

uBot uses a multi-layered security system:

### Security Middleware (`internal/tools/security.go`)

All tool calls pass through `SecureRegistry` — a wrapper around `ToolRegistry`:

- **Sensitive path blocking** — `~/.ssh/`, `~/.gnupg/`, `~/.aws/`, `~/.kube/`, `*.pem`, `*.key`, `.env`, `/etc/shadow`
- **Parameter validation** — `ValidateParams()` checks JSON Schema before every call
- **Exec guard** — integration with `sandbox.GuardCommand()` to block dangerous commands
- **Symlink resolution** — paths are resolved via `filepath.EvalSymlinks` (handles `/etc` -> `/private/etc` on macOS)
- **Audit logging** — all tool calls are logged with timestamps and status

### Sandbox

- **Sandboxed Execution** — commands run in isolated Docker containers
- **gVisor Support** — optional kernel-level isolation
- **Command Guards** — blocks dangerous commands (rm -rf, fork bombs, etc.)
- **Resource Limits** — CPU, memory, and PID limits
- **Non-root Container** — runs as an unprivileged user
- **Read-only Filesystem** — prevents modifications

### Self-Management (CLI Only)

The bot can manage itself via the `manage_ubot` tool, but **only from CLI**:

```
manage_ubot action=show_config     # Show current config
manage_ubot action=update_config key=agents.defaults.model value=gpt-4
manage_ubot action=restart         # Request a restart
```

When called from Telegram/WhatsApp, access is automatically denied with "Permission Denied". Access control is enforced via the `Session.Source` field, which is set automatically for each channel.

## Comparison

| Feature | uBot (Go) | Alternatives (Python) |
|---------|-----------|----------------------|
| Codebase | ~12k lines | 400k+ lines |
| Binary | 15MB | 50MB+ with deps |
| Startup | ~50ms | ~2s |
| Memory | ~20MB | ~100MB |
| Type Safety | Compile-time | Runtime |
| MCP Support | ✅ | ❌ |
| Skill System | ✅ | ✅ |
| Self-contained | ✅ Single binary | ❌ Requires Python |

## Development

```bash
# Build
go build -o ubot ./cmd/ubot/

# Run tests
go test ./...

# Build with version info
go build -ldflags="-X 'main.Version=1.0.0'" ./cmd/ubot/
```

## Uninstall

```bash
ubot destroy
```

Full cleanup includes:
- Docker containers (`ubot`, `ubot-sandboxed`) and **all** images (including versioned tags)
- Configuration and data (`~/.ubot/`)
- CLI command (`~/.local/bin/ubot`)
- PATH entries from shell configs (`~/.zshrc`, `~/.bashrc`, `~/.bash_profile`, `~/.profile`)
- Systemd service on Linux (`/etc/systemd/system/ubot.service`)

---

<p align="center">
  <b>Shipped to you by <a href="https://github.com/lubluniky">Borkiss</a></b>
</p>
