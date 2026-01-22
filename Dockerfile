# Aquitar reverse tunnel proxy server
# Multi-stage build for minimal, secure production image

# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app

# Install git for go mod download (some deps may need it)
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build application
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/aquitar-server ./cmd/aquitar-server

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for TLS, tzdata for timezone support, and netcat for health checks
RUN apk --no-cache add ca-certificates tzdata netcat-openbsd

# Create non-root user
RUN adduser -D -g '' aquitar

# Create directories for cert cache (will be mounted as volume)
RUN mkdir -p /data/certs && chown aquitar:aquitar /data/certs

# Copy binary from builder
COPY --from=builder /app/aquitar-server /usr/local/bin/aquitar-server

# Switch to non-root user
USER aquitar

# Default control port
EXPOSE 8443

# Health check - verify process is running and accepting connections
# Note: This is a TCP service, not HTTP, so we check if port is listening
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD nc -z localhost 8443 || exit 1

# Default cert cache location
ENV AQUITAR_CERT_CACHE=/data/certs

# Entrypoint
ENTRYPOINT ["/usr/local/bin/aquitar-server"]
