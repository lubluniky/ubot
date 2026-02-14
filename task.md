# ТЗ: Предрелизный аудит и тестирование ubot v1.0-beta

**Проект:** ubot (Go, personal AI assistant framework)
**Репо:** `github.com/lubluniky/ubot`
**Ветка:** `main` (коммит `a6e3f2f`)
**Приоритет:** Высокий — блокирует выход в beta production

---

## Часть А: Аудит безопасности и поиск уязвимостей

### Контекст

Уже был проведён security hardening (PR #1), закрывший основные векторы: SSRF, path traversal, file permissions, config redaction, MCP env isolation, allow-list default. Нужен повторный проход — свежим взглядом, с фокусом на то, что могло быть упущено.

### Scope аудита

**Файлы для ревью (по приоритету):**

| Приоритет | Пакет | Файлы | На что смотреть |
|-----------|-------|-------|-----------------|
| CRITICAL | `internal/tools/` | `shell.go`, `filesystem.go`, `web.go`, `browser.go`, `security.go` | Command injection, path traversal, SSRF bypass, обход SecureRegistry |
| CRITICAL | `internal/channels/` | `telegram.go`, `base.go` | Auth bypass, input sanitization, message injection |
| CRITICAL | `internal/agent/` | `loop.go`, `tools/` | Prompt injection → tool execution chain, privilege escalation |
| HIGH | `internal/mcp/` | `client.go` | Env leakage, command injection через server config |
| HIGH | `internal/session/` | `manager.go` | Path traversal (уже фиксили, но перепроверить), session hijacking |
| HIGH | `internal/config/` | `loader.go` | Race conditions при записи, symlink attacks |
| MEDIUM | `internal/sandbox/` | `*.go` | Container escape vectors, resource limits |
| MEDIUM | `internal/providers/` | `litellm_provider.go` | API key leakage в логах/ошибках |
| LOW | `internal/bus/` | `*.go` | Message spoofing между каналами |
| LOW | `internal/cron/` | `*.go` | Cron injection, persistence |

### Конкретные векторы для проверки

**1. Command Injection**
- Проверить все места где пользовательский ввод попадает в `exec.Command` / `exec.CommandContext`
- Shell tool: экранирование аргументов, обход `restrictToWorkspace`
- MCP client: можно ли через конфиг `command`/`args` выполнить произвольное

**2. Path Traversal (глубокий проход)**
- `filesystem.go`: `read_file`, `write_file`, `edit_file`, `list_dir` — все ли проходят через `resolvePath`?
- Symlink following: если в workspace есть symlink на `/etc/shadow` — блокируется ли?
- Unicode / encoding tricks: `%2e%2e%2f`, overlong UTF-8
- `session/manager.go`: `safeKey()` — достаточно ли текущей санитизации? Что с Unicode-символами `／` (fullwidth slash)?

**3. SSRF Bypass**
- DNS rebinding: `isInternalURL()` резолвит IP при валидации, но HTTP-клиент может получить другой IP при повторном резолве
- Redirect-based SSRF: внешний URL делает 302 → `http://169.254.169.254` — проверяется ли redirect target?
- IPv6 bypass: `[::1]`, `[::ffff:127.0.0.1]`, `0x7f000001`
- URL parsing discrepancies: `http://localhost@evil.com`, `http://127.0.0.1:80@external.com`

**4. Prompt Injection → Tool Abuse**
- Может ли содержимое веб-страницы (через `web_fetch`) заставить агента вызвать `shell_exec rm -rf`?
- Может ли Telegram-сообщение содержать instruction injection?
- Есть ли rate limiting на tool calls?

**5. Authentication / Authorization**
- `IsAllowed()` — compound ID парсинг (`123456|username`): можно ли подделать?
- `ManageUbotTool` — проверка `source == "cli"`: можно ли подменить source?
- Gateway HTTP endpoint — есть ли auth?

**6. Information Disclosure**
- Error messages: не утекают ли пути, API ключи, стектрейсы?
- `redactSensitiveFields()`: поля `api_base` с inline credentials (`http://key:secret@host`) — редактируются?
- Logging: что пишется в stdout/stderr? Нет ли чувствительных данных?

**7. Race Conditions**
- `session/manager.go`: concurrent Save/Load на одну сессию
- `config/loader.go`: concurrent SaveConfig
- Tool registry: concurrent Execute calls

### Deliverables по Части А

1. **Отчёт** со списком найденных проблем, severity (Critical / High / Medium / Low / Info), и рекомендациями по фиксу
2. **PoC** для каждой уязвимости выше Medium (конкретный input → ожидаемый bad outcome)
3. PR с фиксами для всего что Critical/High

---

## Часть Б: Мануальное тестирование

### Настройка тестового окружения

```bash
# 1. Клонировать и собрать
git clone github.com/lubluniky/ubot && cd ubot
go build -o ubot ./cmd/ubot/

# 2. Создать тестовый конфиг
./ubot status  # инициализирует ~/.ubot/

# 3. Настроить минимальный провайдер (OpenAI или любой другой)
# Вписать API key в ~/.ubot/config.json → providers.openai.api_key

# 4. Для Telegram-тестов: создать тест-бота через @BotFather, вписать token
```

### Чеклист

Отмечать каждый пункт: **PASS** / **FAIL** / **SKIP** (с причиной)

---

#### 1. Telegram канал

| # | Тест | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 1.1 | Старт gateway без `allow_from` в конфиге | WARNING в логе, все входящие сообщения отклоняются | |
| 1.2 | Старт с заполненным `allow_from` | Только указанные user ID получают ответы | |
| 1.3 | Сообщение от неавторизованного user ID | Отклоняется, в логе `[security] channel=telegram action=denied sender=XXXXX` | |
| 1.4 | Голосовое сообщение (если настроен voice) | Транскрипция работает, агент получает текст | |
| 1.5 | Очень длинное сообщение (>4096 символов) | Не ломает парсинг, обрабатывается корректно | |
| 1.6 | Сообщение с Markdown/HTML спецсимволами | Не ломает ответ бота | |

#### 2. SSRF Protection

| # | Тест | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 2.1 | `web_fetch` → `http://localhost:8080` | Блокируется: "internal/private network" | |
| 2.2 | `web_fetch` → `http://127.0.0.1` | Блокируется | |
| 2.3 | `web_fetch` → `http://169.254.169.254/latest/meta-data` | Блокируется (cloud metadata) | |
| 2.4 | `web_fetch` → `http://192.168.1.1` | Блокируется (private range) | |
| 2.5 | `web_fetch` → `http://10.0.0.1` | Блокируется (private range) | |
| 2.6 | `web_fetch` → `https://example.com` | Работает нормально | |
| 2.7 | `browser_use browse_page` → localhost | Блокируется | |
| 2.8 | `browser_use browse_page` → внешний URL | Работает | |
| 2.9 | `web_fetch` → `http://[::1]/` (IPv6 loopback) | Блокируется | |

#### 3. Config Redaction

| # | Тест | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 3.1 | `manage_ubot show_config` (с заполненными API keys) | Ключи показаны как `****XXXX` (последние 4 символа) | |
| 3.2 | `manage_ubot show_config` (пустые ключи) | Пустые строки, не `****` | |
| 3.3 | Вывод — валидный JSON | Парсится без ошибок | |
| 3.4 | Все поля конфига присутствуют (не потерялись при redaction) | Структура полная | |

#### 4. File Permissions

| # | Тест | Как проверить | Ожидаемый результат | Статус |
|---|------|--------------|---------------------|--------|
| 4.1 | Config dir | `stat ~/.ubot/` | `drwx------` (0700) | |
| 4.2 | Config file | `stat ~/.ubot/config.json` | `-rw-------` (0600) | |
| 4.3 | Workspace dir | `stat ~/.ubot/workspace/` | `drwx------` (0700) | |
| 4.4 | Sessions dir | `stat ~/.ubot/sessions/` | `drwx------` (0700) | |
| 4.5 | Session file (после диалога) | `stat ~/.ubot/sessions/*.json` | `-rw-------` (0600) | |
| 4.6 | MEMORY.md (после записи) | `stat ~/.ubot/workspace/MEMORY.md` | `-rw-------` (0600) | |
| 4.7 | Daily notes (после записи) | `stat ~/.ubot/workspace/memory/*.md` | `-rw-------` (0600) | |

#### 5. Session Path Traversal

| # | Тест | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 5.1 | Session key с `../../etc/passwd` | Sanitized, файл создаётся внутри sessions/ | |
| 5.2 | Session key с null bytes (`\x00`) | Очищается, нет ошибки | |
| 5.3 | Session key с backslash (`\`) | Очищается | |
| 5.4 | Session key с двоеточием (`:`) | Заменяется на `_` | |

#### 6. MCP Environment Isolation

| # | Тест | Как проверить | Ожидаемый результат | Статус |
|---|------|--------------|---------------------|--------|
| 6.1 | Запустить MCP сервер (например `env` как command) | Посмотреть вывод | Только whitelist переменных: PATH, HOME, LANG, USER, TERM, SHELL, TMPDIR, XDG_RUNTIME_DIR, NODE_PATH | |
| 6.2 | Проверить что API ключи не видны | `env \| grep -i key` в MCP subprocess | Пусто | |
| 6.3 | Explicit env vars в конфиге MCP сервера | Настроить `"env": {"FOO": "bar"}` | FOO=bar присутствует | |

#### 7. Sensitive File Blocking

| # | Тест | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 7.1 | `read_file ~/.ubot/config.json` | Заблокировано SecureRegistry | |
| 7.2 | `read_file ~/.ssh/id_rsa` | Заблокировано | |
| 7.3 | `read_file /proc/self/environ` | Заблокировано | |
| 7.4 | `read_file /proc/self/cmdline` | Заблокировано | |
| 7.5 | `list_dir ~/.ssh/` | Заблокировано | |
| 7.6 | `read_file ~/.ubot/workspace/notes.md` | Разрешено | |
| 7.7 | `write_file` в `.env` файл | Заблокировано | |

#### 8. Docker / Sandbox

| # | Тест | Как проверить | Ожидаемый результат | Статус |
|---|------|--------------|---------------------|--------|
| 8.1 | `docker-compose up -d` | `ss -tlnp \| grep 18790` или `netstat` | Слушает на `127.0.0.1:18790`, НЕ на `0.0.0.0` | |
| 8.2 | Sandbox image | `docker inspect` контейнера | `alpine:3.21` | |
| 8.3 | `docker-compose down` | Все контейнеры остановлены | Чисто, без orphans | |
| 8.4 | Sandbox memory limit | Запустить `stress` в sandbox | Убивается при превышении 128MB | |
| 8.5 | Sandbox timeout | Запустить `sleep 300` | Прерывается по timeout (30s default) | |

#### 9. Функциональные проверки (smoke tests)

| # | Тест | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 9.1 | `ubot agent -m "привет"` | Ответ от LLM, без ошибок | |
| 9.2 | `ubot agent` (interactive mode) | Запускается REPL, можно вести диалог | |
| 9.3 | `ubot gateway` | Стартует, в логе подключение каналов | |
| 9.4 | `ubot status` | Показывает текущий конфиг и состояние | |
| 9.5 | Tool: `shell_exec echo hello` | Возвращает "hello" | |
| 9.6 | Tool: `read_file` / `write_file` в workspace | Читает/пишет корректно | |
| 9.7 | Tool: `web_search` (если настроен) | Возвращает результаты | |
| 9.8 | Memory: агент пишет в MEMORY.md | Файл обновляется, контент сохраняется | |
| 9.9 | Memory: daily notes | Создаётся файл с текущей датой | |
| 9.10 | Cron: создание задачи | `cron_manage add ...` — задача добавляется | |
| 9.11 | Cron: выполнение по расписанию | Задача срабатывает в указанное время | |
| 9.12 | Cron: удаление задачи | Задача удаляется, больше не срабатывает | |
| 9.13 | Skills: загрузка и выполнение skill | Skill загружается из `.md`, агент использует инструкции | |

#### 10. Regression / обратная совместимость

| # | Тест | Ожидаемый результат | Статус |
|---|------|---------------------|--------|
| 10.1 | Загрузка старого конфига (без новых полей) | Defaults подхватываются, без panic | |
| 10.2 | Загрузка существующих сессий | Диалоги загружаются корректно | |
| 10.3 | MCP серверы с env-зависимостями | Проверить что NODE_PATH и PATH достаточно; если сервер требует другие env — задокументировать | |
| 10.4 | Обновление с предыдущей версии | `go install` поверх старой — всё работает | |

---

## Порядок выполнения

1. **Сначала Часть А** (аудит) — может выявить баги, которые влияют на тест-план
2. **Потом пункты 9.1–9.4** (smoke tests) — убедиться что базовый функционал работает
3. **Потом Security тесты** (пункты 2–7) — самое критичное для beta
4. **Потом Docker + Telegram** (пункты 1, 8)
5. **В конце Regression** (пункт 10)

## Критерии прохождения

- **БЛОКЕР для релиза:** любой FAIL в пунктах 2–7 (security)
- **БЛОКЕР:** любой FAIL в 9.1–9.4 (базовый функционал)
- **Высокий приоритет:** FAIL в пунктах 1, 8 (каналы, Docker)
- **Можно релизить с known issues:** FAIL в 9.10–9.13, 10.3–10.4
