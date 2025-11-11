# Multi-stage Dockerfile for reddit-server
# Uses Alpine Linux for minimal image size while supporting SQLite CGO

# ============================================
# Builder Stage
# ============================================
FROM golang:1.24-alpine AS builder

# Install build dependencies for CGO (required for mattn/go-sqlite3)
RUN apk add --no-cache \
    gcc \
    musl-dev \
    git

# Set working directory
WORKDIR /build

# Copy source code (includes go.mod and go.sum)
COPY . .

# Download dependencies
RUN go mod download

# Build the reddit-server binary
# CGO is enabled by default on Alpine, needed for SQLite
RUN go build \
    -ldflags="-w -s" \
    -o reddit-server \
    ./cmd/reddit-server

# Verify the binary was built
RUN test -f reddit-server || (echo "Build failed: binary not found" && exit 1)

# ============================================
# Runtime Stage
# ============================================
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata

# Create non-root user and group
RUN addgroup -g 1000 reddit && \
    adduser -u 1000 -G reddit -s /bin/sh -D reddit

# Create directories for data and logs
RUN mkdir -p /data /logs && \
    chown -R reddit:reddit /data /logs

# Copy binary from builder
COPY --from=builder /build/reddit-server /usr/local/bin/reddit-server

# Set working directory
WORKDIR /app

# Switch to non-root user
USER reddit

# Expose HTTP port
EXPOSE 8080

# Configure health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set default environment variables
ENV PORT=8080 \
    STORAGE_DSN=/data/reddit.db \
    LOG_LEVEL=info \
    LOG_FORMAT=json

# Run the server
ENTRYPOINT ["/usr/local/bin/reddit-server"]
