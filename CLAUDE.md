# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**uBot** is a Go rewrite of [nanobot](../), an ultra-lightweight self-hosted AI assistant. Single binary (~15MB), ~50ms startup, ~20MB RAM. Multi-channel support (Telegram, WhatsApp, CLI), tool execution, skills system, MCP integration, and Docker sandboxing.

Module: `github.com/hkuds/ubot` — Go 1.25.5

## Build & Development Commands

```bash
# Build
go build -o ubot ./cmd/ubot/

# Run tests
go test ./...
go test ./internal/tools/ -v              # single package
go test ./internal/tools/ -run TestSecure  # single test pattern

# Run
./ubot agent                # interactive CLI chat
./ubot agent -m "message"   # single message
./ubot gateway              # start Telegram/WhatsApp channels
./ubot setup                # interactive setup wizard
./ubot status               # show config
./ubot skills               # manage skills
./ubot version              # show version

# Docker
docker build -t ubot .
docker compose up -d
```

There is no linter configured in CI; the codebase uses standard `go vet`.

## Architecture

```
Channels (Telegram, WhatsApp, CLI)
        ↓ InboundMessage
   MessageBus (internal/bus/) — async channel-based queue
        ↓
   AgentLoop (internal/agent/loop.go)
   ├── ContextBuilder (context.go) — system prompt from workspace files
   ├── Provider (internal/providers/) — LLM calls via OpenAI-compatible API
   ├── SecureRegistry (internal/tools/) — security-wrapped tool execution
   └── SessionManager (internal/session/) — JSONL conversation history
        ↓ OutboundMessage
   MessageBus → Channels
```

### Agent loop flow (`ProcessMessage`)

1. Get/create session by `channel:chatid` key
2. Build messages: system prompt (from workspace files) + history + user input
3. Loop up to `maxToolIterations` (default 10): call LLM → if tool calls, execute via SecureRegistry → append results → repeat
4. Final LLM call without tools to get text response
5. Save to session, return OutboundMessage

### Two execution paths

The **gateway** path (`cmd/ubot/cmd/gateway.go`) uses the full `agent.Loop` with `MessageBus` for multi-channel routing. The **CLI agent** path (`cmd/ubot/cmd/agent.go`) bypasses `agent.Loop` and drives the LLM/tool loop directly in `sendSingleMessage()` and `runInteractiveMode()`. Both paths share the same tool registry and security middleware.

### Key interfaces

**Tool** (`internal/tools/base.go`): `Name()`, `Description()`, `Parameters()` (JSON Schema), `Execute(ctx, params) (string, error)`. All tools validate params via `ValidateParams()` against their JSON schema.

**Provider** (`internal/providers/base.go`): `Name()`, `Chat(ctx, ChatRequest) (*ChatResponse, error)`, `DefaultModel()`. All providers use the OpenAI-compatible API format. Factory in `factory.go` auto-selects first available by priority: Copilot → OpenRouter → Anthropic → OpenAI → Gemini → Groq → VLLM.

**Channel** (`internal/channels/base.go`): `Name()`, `Start(ctx)`, `Stop()`, `Send(OutboundMessage)`, `IsRunning()`.

### Security layers

1. **SecureRegistry** (`tools/security.go`) — blocks access to sensitive paths (~/.ssh, ~/.aws, .env, .pem, etc.) with symlink resolution
2. **Shell guards** (`tools/shell.go`) — regex-based blocking of destructive commands (rm -rf /, fork bombs, dd to devices, etc.)
3. **Sandbox** (`internal/sandbox/`) — Docker container isolation with optional gVisor, non-root user, resource limits
4. **Channel allowlists** — `allowFrom` config per channel

## Adding a New Tool

1. Create `internal/tools/yourtool.go` implementing the `Tool` interface
2. Register it in `registerDefaultTools()` in `cmd/ubot/cmd/agent.go` (for CLI) and in `runGateway()` in `cmd/ubot/cmd/gateway.go` (for gateway mode)
3. The SecureRegistry automatically wraps it with security checks

## Configuration

User data lives in `~/.ubot/`. Config at `~/.ubot/config.json` with providers, channels, tools, and MCP server definitions. Sessions stored as JSONL in `~/.ubot/workspace/sessions/`. System prompt assembled from workspace markdown files: `AGENTS.md`, `SOUL.md`, `USER.md`, `TOOLS.md`, plus memory context.

## Test Conventions

- Table-driven tests with `[]struct` and `t.Run()` subtests
- Test files colocated in same package (`*_test.go`)
- Mock HTTP clients for provider/API tests
