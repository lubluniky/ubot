# uBot

**Ultra-lightweight personal AI assistant** - A complete rewrite of [nanobot](https://github.com/HKUDS/nanobot) in Go for better performance, type safety, and containerization.

## Features

- **Multiple LLM Providers**: OpenRouter, Anthropic, OpenAI, GitHub Copilot, Groq, Gemini, Ollama/vLLM
- **GitHub Copilot Integration**: Device flow authentication, uses your existing Copilot subscription
- **Multi-Channel Support**: Telegram, WhatsApp (coming soon), CLI
- **Tool Execution**: File operations, shell commands, web search, web fetch
- **Secure Sandboxing**: Docker-based command execution with gVisor support
- **Interactive TUI**: Beautiful setup wizard using Charm libraries
- **Memory System**: Daily notes and long-term memory persistence
- **Session Management**: Conversation history per channel/chat

## Quick Start

### Install from Source

```bash
git clone https://github.com/hkuds/ubot.git
cd ubot
go build -o ubot ./cmd/ubot/
./ubot setup  # Interactive setup wizard
```

### Run with Docker

```bash
# Build image
docker build -t ubot .

# Initialize config (first time)
docker run -it -v ~/.ubot:/home/ubot/.ubot ubot setup

# Run gateway (connects to Telegram)
docker run -d -v ~/.ubot:/home/ubot/.ubot -p 18790:18790 ubot
```

### Docker Compose

```bash
# Run with docker compose
docker compose up -d

# With gVisor sandboxing (requires gVisor installed)
docker compose --profile sandboxed up -d
```

## CLI Commands

```bash
ubot setup      # Interactive setup wizard
ubot agent      # Interactive chat mode
ubot agent -m "Hello"  # Single message mode
ubot gateway    # Start channel gateway (Telegram, etc.)
ubot status     # Show configuration status
ubot version    # Show version info
```

## Configuration

Config file: `~/.ubot/config.json`

```json
{
  "agents": {
    "defaults": {
      "model": "anthropic/claude-opus-4-5",
      "maxTokens": 8192,
      "temperature": 0.7
    }
  },
  "providers": {
    "copilot": {
      "enabled": true,
      "accessToken": "gho_xxx"
    },
    "openrouter": {
      "apiKey": "sk-or-v1-xxx"
    }
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
      "search": {
        "apiKey": "BSA..."
      }
    }
  }
}
```

## GitHub Copilot Setup

uBot supports GitHub Copilot as an LLM provider. To use it:

1. Run `ubot setup` and select "GitHub Copilot"
2. Visit the URL shown and enter the code
3. Authorize the application
4. Your Copilot subscription is now connected!

## Provider Priority

When multiple providers are configured, uBot uses this priority:

1. GitHub Copilot (if enabled)
2. OpenRouter
3. Anthropic
4. OpenAI
5. Gemini
6. Groq
7. vLLM/Ollama

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      CHANNELS LAYER                          │
│  (Telegram, WhatsApp, CLI) - Message I/O Gateways          │
└──────────────┬──────────────────────────────────────────────┘
               │
               ▼
      ┌────────────────────┐
      │  Message Bus       │  (Async queue-based routing)
      │  (inbound/outbound)│
      └────────┬───────────┘
               │
               ▼
┌──────────────────────────────────────────────────────────────┐
│                    AGENT LOOP (Core Engine)                  │
│  ├─ Context Builder    (Prompts from workspace files)       │
│  ├─ LLM Provider       (Multi-provider support)             │
│  ├─ Tool Registry      (File, Shell, Web, Message)          │
│  ├─ Session Manager    (Conversation history)               │
│  └─ Memory System      (Daily + long-term memory)           │
└──────────────────────────────────────────────────────────────┘
```

## Project Structure

```
ubot/
├── cmd/ubot/           # CLI entry point
│   └── cmd/            # Cobra commands
├── internal/
│   ├── agent/          # Agent loop, context, memory
│   ├── bus/            # Message bus (inbound/outbound)
│   ├── channels/       # Telegram, WhatsApp, CLI
│   ├── config/         # Configuration schema & loader
│   ├── providers/      # LLM providers (OpenAI, Copilot, etc.)
│   ├── sandbox/        # Docker-based sandboxing
│   ├── session/        # Conversation sessions
│   ├── tools/          # Agent tools (file, shell, web)
│   └── tui/            # Terminal UI (setup wizard)
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

## Security

- **Sandboxed Execution**: Shell commands run in isolated Docker containers
- **gVisor Support**: Optional kernel-level isolation with gVisor runtime
- **Command Guards**: Dangerous commands (rm -rf, fork bombs, etc.) are blocked
- **Resource Limits**: CPU, memory, and PID limits on sandboxed processes
- **Non-root Container**: Runs as unprivileged user in Docker

## Development

```bash
# Build
go build -o ubot ./cmd/ubot/

# Run tests
go test ./...

# Lint
golangci-lint run

# Build with version info
go build -ldflags="-X 'github.com/hkuds/ubot/cmd/ubot/cmd.Version=1.0.0'" ./cmd/ubot/
```

## Comparison with nanobot (Python)

| Feature | nanobot (Python) | uBot (Go) |
|---------|------------------|-----------|
| Binary size | ~50MB (with deps) | ~15MB |
| Startup time | ~2s | ~50ms |
| Memory usage | ~100MB | ~20MB |
| Type safety | Runtime | Compile-time |
| GitHub Copilot | No | Yes |
| TUI Setup | No | Yes |
| Sandboxing | Basic | Docker+gVisor |
| Cross-platform | Requires Python | Single binary |

## License

MIT
