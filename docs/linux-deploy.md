# Linux Production Deployment with Headless Chromium

## Install Chromium

### Ubuntu / Debian

```bash
apt-get update && apt-get install -y \
  chromium-browser \
  libnss3 libatk-bridge2.0-0 libx11-xcb1 libxcomposite1 \
  libxdamage1 libxrandr2 libgbm1 libasound2 libpangocairo-1.0-0 \
  libgtk-3-0 fonts-liberation fonts-noto-color-emoji
```

### Alpine

```bash
apk add --no-cache chromium nss freetype harfbuzz ca-certificates ttf-freefont
```

## Build uBot

```bash
go build -o ubot ./cmd/ubot/
```

## Configuration

Add browser settings to `~/.ubot/config.json`:

```json
{
  "tools": {
    "browser": {
      "sessionDir": "/home/ubot/.ubot/workspace/browser-sessions",
      "proxy": "",
      "stealth": true,
      "idleTimeout": 300
    }
  }
}
```

### Proxy examples

```json
{ "proxy": "http://proxy.local:8080" }
{ "proxy": "socks5://127.0.0.1:1080" }
{ "proxy": "https://user:pass@proxy.example.com:3128" }
```

## Systemd Service

Create `/etc/systemd/system/ubot.service`:

```ini
[Unit]
Description=uBot AI Assistant Gateway
After=network.target

[Service]
Type=simple
User=ubot
Group=ubot
WorkingDirectory=/home/ubot
ExecStart=/usr/local/bin/ubot gateway
Restart=on-failure
RestartSec=5
Environment=HOME=/home/ubot

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ubot
sudo journalctl -u ubot -f
```

## Docker Deployment

### Dockerfile

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /ubot ./cmd/ubot/

FROM alpine:3.21
RUN apk add --no-cache chromium nss freetype harfbuzz ca-certificates ttf-freefont
RUN adduser -D -h /home/ubot ubot
USER ubot
WORKDIR /home/ubot
COPY --from=builder /ubot /usr/local/bin/ubot
ENTRYPOINT ["ubot"]
CMD ["gateway"]
```

### docker-compose.yml

```yaml
services:
  ubot:
    build: .
    restart: unless-stopped
    volumes:
      - ubot-data:/home/ubot/.ubot
    environment:
      - HOME=/home/ubot
    # Optional: expose gateway HTTP port
    # ports:
    #   - "8080:8080"

volumes:
  ubot-data:
```

## Session Directory

Browser sessions are stored in `~/.ubot/workspace/browser-sessions/`. Each named session is a Chrome user-data-dir containing cookies, localStorage, and other profile data.

Ensure correct permissions:

```bash
chmod 700 ~/.ubot/workspace/browser-sessions
```

To back up sessions:

```bash
tar czf browser-sessions-backup.tar.gz -C ~/.ubot/workspace browser-sessions/
```

## Troubleshooting

**Chrome fails to start**: Ensure `--no-sandbox` is allowed (runs as non-root user in Docker) or run with `--cap-add=SYS_ADMIN` in Docker.

**Missing shared libraries**: Run `ldd $(which chromium)` to find missing `.so` files and install the corresponding packages.

**`/dev/shm` too small in Docker**: The `--disable-dev-shm-usage` flag is set by default, which makes Chrome use `/tmp` instead. If you still see crashes, increase shared memory: `--shm-size=256m` in Docker.
