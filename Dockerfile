# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with version info
ARG VERSION=0.1.0
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X 'github.com/hkuds/ubot/cmd/ubot/cmd.Version=${VERSION}' \
    -X 'github.com/hkuds/ubot/cmd/ubot/cmd.GitCommit=${GIT_COMMIT}' \
    -X 'github.com/hkuds/ubot/cmd/ubot/cmd.BuildDate=${BUILD_DATE}'" \
    -o /ubot ./cmd/ubot/

# Runtime stage - minimal secure image
FROM alpine:3.21

# Security: Run as non-root user
RUN addgroup -S ubot && adduser -S ubot -G ubot

# Install ca-certificates for HTTPS and tzdata for timezone
RUN apk add --no-cache ca-certificates tzdata

# Copy binary
COPY --from=builder /ubot /usr/local/bin/ubot

# Create data directories
RUN mkdir -p /home/ubot/.ubot/workspace/memory \
    && mkdir -p /home/ubot/.ubot/sessions \
    && chown -R ubot:ubot /home/ubot/.ubot

# Switch to non-root user
USER ubot
WORKDIR /home/ubot

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ubot status || exit 1

# Default command
ENTRYPOINT ["ubot"]
CMD ["gateway"]

# Expose gateway port
EXPOSE 18790

# Labels
LABEL org.opencontainers.image.title="uBot"
LABEL org.opencontainers.image.description="Ultra-lightweight personal AI assistant"
LABEL org.opencontainers.image.source="https://github.com/hkuds/ubot"
LABEL org.opencontainers.image.licenses="MIT"
