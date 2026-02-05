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

**uBot** — самый легковесный self-hosted AI ассистент в мире. Полная переписка [nanobot](https://github.com/HKUDS/nanobot) на Go для максимальной производительности и безопасности.

[![GitHub](https://img.shields.io/badge/GitHub-lubluniky%2Fubot-blue?logo=github)](https://github.com/lubluniky/ubot)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

## Features

- **Ultra-Lightweight** — ~12,000 строк Go кода (vs 400k+ у аналогов)
- **Self-Hosted** — данные остаются на твоём железе
- **Multi-Provider** — OpenRouter, GitHub Copilot, Anthropic, OpenAI, Ollama
- **Multi-Channel** — Telegram, WhatsApp (скоро), CLI
- **Tool System** — файлы, shell, web search, web fetch
- **Skill System** — расширяй возможности через SKILL.md файлы
- **MCP Support** — подключай внешние инструменты через Model Context Protocol
- **Secure Sandbox** — Docker-based изоляция с gVisor поддержкой
- **Interactive TUI** — красивый setup wizard

## Quick Start

### One-Line Install

```bash
curl -fsSL https://raw.githubusercontent.com/lubluniky/ubot/main/install.sh | bash
```

Установщик:
- Проверит OS и зависимости
- Установит Docker если нужно
- Соберёт Docker образ
- Запустит интерактивную настройку
- Создаст команду `ubot`

### Manual Install

```bash
git clone https://github.com/lubluniky/ubot.git
cd ubot
go build -o ubot ./cmd/ubot/
./ubot setup
```

## Usage

```bash
ubot start       # Запустить gateway (Telegram, etc.)
ubot stop        # Остановить gateway
ubot restart     # Перезапустить
ubot logs        # Показать логи
ubot status      # Показать конфигурацию
ubot chat        # Интерактивный чат
ubot chat -m "Hello!"  # Одно сообщение
ubot setup       # Мастер настройки
ubot config      # Редактировать конфиг
ubot update      # Обновить до последней версии
ubot destroy     # Полное удаление
```

## Configuration

Конфиг: `~/.ubot/config.json`

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
    }
  },
  "mcp": {
    "servers": []
  }
}
```

## Providers

| Provider | Описание | API Key |
|----------|----------|---------|
| **OpenRouter** | Доступ к Claude, GPT-4, Llama | [openrouter.ai/keys](https://openrouter.ai/keys) |
| **GitHub Copilot** | Бесплатно с подпиской GitHub | Device Flow в setup |
| **Anthropic** | Claude напрямую | [console.anthropic.com](https://console.anthropic.com) |
| **OpenAI** | GPT-4 напрямую | [platform.openai.com](https://platform.openai.com) |
| **Ollama** | Локальные модели | Не нужен |

## Skills

Скиллы расширяют возможности бота. Создай `~/.ubot/workspace/skills/{name}/SKILL.md`:

```markdown
# Code Review

Помогаю ревьюить код на баги и улучшения.

<!-- always-load -->

## Capabilities

- Поиск багов
- Проверка безопасности
- Предложения по улучшению

## Tools

- `read_file`: читать файлы для анализа
- `exec`: запускать линтеры
```

Бот автоматически найдёт и предложит использовать скиллы.

## MCP (Model Context Protocol)

Подключай внешние инструменты через MCP:

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

MCP инструменты появятся как `mcp_{server}_{tool}` в списке доступных.

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
│   ├── mcp/            # MCP client & manager
│   ├── providers/      # LLM providers
│   ├── sandbox/        # Docker sandboxing
│   ├── session/        # Conversation sessions
│   ├── skills/         # Skill loader & parser
│   ├── tools/          # Built-in tools
│   └── tui/            # Terminal UI
├── skills/             # Bundled skills
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

- **Sandboxed Execution** — команды выполняются в изолированных Docker контейнерах
- **gVisor Support** — опциональная kernel-level изоляция
- **Command Guards** — блокировка опасных команд (rm -rf, fork bombs, etc.)
- **Resource Limits** — лимиты CPU, памяти, PID
- **Non-root Container** — запуск от непривилегированного пользователя
- **Read-only Filesystem** — защита от модификации

## Comparison

| Feature | uBot (Go) | Аналоги (Python) |
|---------|-----------|------------------|
| Размер кода | ~12k строк | 400k+ строк |
| Бинарник | 15MB | 50MB+ с deps |
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

Это удалит:
- Docker контейнер и образ
- Конфигурацию (~/.ubot/)
- Команду ubot

---

<p align="center">
  <b>Shipped to you by <a href="https://github.com/lubluniky">Borkiss</a></b>
</p>
