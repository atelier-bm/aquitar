# Aquitar

**Aquitar** is a reverse tunnel proxy that exposes internal services to the internet through secure, multiplexed connections.

Perfect for exposing local development servers, IoT devices, or services behind NAT/firewalls without complex network configuration.

## Features

- **Simple Go API** - `net.Listener` interface, drop-in replacement for `net.Listen`
- **Secure by default** - TLS with automatic Let's Encrypt certificates
- **Multiplexed** - Multiple connections over a single TLS session (yamux)
- **PSK authentication** - Pre-shared key for client authorization
- **Docker ready** - Production-ready container with health checks
- **Port management** - Configurable allowed/disallowed ports
- **Zero configuration** - Sensible defaults for quick starts

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
  - [Server Setup](#server-setup)
  - [Client Usage](#client-usage)
- [Usage](#usage)
  - [Running the Server](#running-the-server)
  - [Client Library](#client-library)
  - [Docker Deployment](#docker-deployment)
- [Configuration](#configuration)
  - [Server Configuration](#server-configuration)
  - [Client Configuration](#client-configuration)
- [Examples](#examples)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Development](#development)

## Installation

### Server Binary

```bash
go install github.com/atelier-bm/aquitar/cmd/aquitar-server@latest
```

### Client Library

```bash
go get github.com/atelier-bm/aquitar
```

### Docker Image

```bash
docker pull ghcr.io/atelier-bm/aquitar:latest
```

## Quick Start

### Server Setup

**1. Generate a pre-shared key:**

```bash
PSK=$(openssl rand -hex 32)
echo "Your PSK: $PSK"
```

**2. Run the server:**

```bash
# Development (no TLS)
aquitar-server -psk "$PSK"

# Production (with TLS)
aquitar-server -domain tunnel.example.com -psk "$PSK"
```

The server listens on port 8443 by default.

### Client Usage

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/atelier-bm/aquitar"
)

func main() {
    // Connect to the tunnel server
    ln, err := aquitar.Listen(context.Background(), aquitar.Config{
        Server: "tunnel.example.com:8443",
        PSK:    "your-psk-here",
        Port:   9001,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer ln.Close()

    // Use it like any net.Listener
    log.Printf("Serving on public port 9001")
    http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello from behind NAT!\n"))
    }))
}
```

Your local service is now accessible at `tunnel.example.com:9001`!

## Usage

### Running the Server

#### Command Line Flags

```bash
aquitar-server [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `-psk` | Pre-shared key (required) | - |
| `-domain` | Domain name for TLS certificates | - |
| `-port` | Control port | 8443 |
| `-config` | Path to JSON configuration file | - |

#### Examples

**Development mode (no TLS):**

```bash
aquitar-server -psk my-secret-key
```

**Production with TLS:**

```bash
aquitar-server -domain tunnel.example.com -psk my-secret-key
```

**Using configuration file:**

```bash
aquitar-server -config config.json
```

#### Configuration File

Create `config.json`:

```json
{
  "domain": "tunnel.example.com",
  "psk": "your-secret-key",
  "control_port": 8443,
  "disallowed_ports": [22, 80, 443, 8443],
  "cert_cache": "/var/lib/aquitar/certs"
}
```

Then run:

```bash
aquitar-server -config config.json
```

**Note:** Command-line flags override config file values.

### Client Library

#### Basic Usage

```go
import (
    "context"
    "github.com/atelier-bm/aquitar"
)

ln, err := aquitar.Listen(ctx, aquitar.Config{
    Server: "tunnel.example.com:8443",
    PSK:    "your-psk-here",
    Port:   9001,
})
if err != nil {
    // Handle error
}
defer ln.Close()

// Use ln as a normal net.Listener
```

#### With Context Cancellation

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

ln, err := aquitar.Listen(ctx, aquitar.Config{
    Server: "tunnel.example.com:8443",
    PSK:    "your-psk-here",
    Port:   9001,
})
if err != nil {
    log.Fatal(err)
}
defer ln.Close()

// Cancel context to close connection
go func() {
    time.Sleep(30 * time.Second)
    cancel() // Closes the tunnel
}()

// Accept connections...
```

#### Error Handling

```go
ln, err := aquitar.Listen(ctx, cfg)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "invalid psk"):
        log.Fatal("Authentication failed: incorrect PSK")
    case strings.Contains(err.Error(), "port in use"):
        log.Fatal("Port already claimed by another client")
    case strings.Contains(err.Error(), "port disallowed"):
        log.Fatal("Port not allowed by server")
    default:
        log.Fatalf("Connection failed: %v", err)
    }
}
```

### Docker Deployment

#### Using Docker Compose

Create `docker-compose.yml`:

```yaml
services:
  aquitar:
    image: ghcr.io/atelier-bm/aquitar:latest
    ports:
      - "8443:8443"   # Control port
      - "9001:9001"   # Client port
      - "9002:9002"   # Add as needed
    environment:
      - AQUITAR_DOMAIN=tunnel.example.com
      - AQUITAR_PSK=your-secret-key
    volumes:
      - cert_cache:/data/certs
    restart: unless-stopped

volumes:
  cert_cache:
```

Run:

```bash
docker compose up -d
```

#### Using Docker Run

```bash
docker run -d \
  --name aquitar \
  -p 8443:8443 \
  -p 9001:9001 \
  -e AQUITAR_DOMAIN=tunnel.example.com \
  -e AQUITAR_PSK=your-secret-key \
  -v aquitar-certs:/data/certs \
  ghcr.io/atelier-bm/aquitar:latest
```

#### Using Configuration File with Docker

Create `config.json`:

```json
{
  "domain": "tunnel.example.com",
  "psk": "your-secret-key",
  "control_port": 8443,
  "disallowed_ports": [22, 80, 443, 8443],
  "cert_cache": "/data/certs"
}
```

Mount and use it:

```bash
docker run -d \
  --name aquitar \
  -p 8443:8443 \
  -p 9001-9010:9001-9010 \
  -v $(pwd)/config.json:/config.json:ro \
  -v aquitar-certs:/data/certs \
  ghcr.io/atelier-bm/aquitar:latest \
  -config /config.json
```

## Configuration

### Server Configuration

#### Environment Variables (Docker)

| Variable | Description | Default |
|----------|-------------|---------|
| `AQUITAR_DOMAIN` | Domain name for TLS | - |
| `AQUITAR_PSK` | Pre-shared key | - |
| `AQUITAR_CERT_CACHE` | Certificate cache directory | `/data/certs` |

#### Configuration Options

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `domain` | string | Domain for TLS certificates | - |
| `psk` | string | Pre-shared key (required) | - |
| `control_port` | int | Port for client connections | 8443 |
| `disallowed_ports` | []int | Ports clients cannot request | [control_port] |
| `cert_cache` | string | TLS certificate cache directory | `~/.cache/aquitar/certs` |

#### Port Management

By default, the control port is automatically disallowed. To restrict additional ports:

```json
{
  "disallowed_ports": [22, 80, 443, 8443]
}
```

This prevents clients from requesting SSH, HTTP, HTTPS, or the control port.

### Client Configuration

#### Config Struct

```go
type Config struct {
    // Server is the address of the proxy server
    // Format: "hostname:port" or "ip:port"
    Server string

    // PSK is the pre-shared key for authentication
    PSK string

    // Port is the public port to request from the server
    // Must be in range 1-65535 and not disallowed by server
    Port int
}
```

#### Validation

The client validates:
- Server address is not empty
- Port is in valid range (1-65535)
- PSK authentication with server

## Examples

### Example 1: Expose HTTP Server

```go
package main

import (
    "context"
    "log"
    "net/http"

    "github.com/atelier-bm/aquitar"
)

func main() {
    ln, err := aquitar.Listen(context.Background(), aquitar.Config{
        Server: "tunnel.example.com:8443",
        PSK:    "my-secret-key",
        Port:   9001,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer ln.Close()

    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello from Aquitar!\n"))
    })

    log.Println("HTTP server listening on public port 9001")
    log.Fatal(http.Serve(ln, mux))
}
```

### Example 2: TCP Echo Server

```go
package main

import (
    "context"
    "io"
    "log"

    "github.com/atelier-bm/aquitar"
)

func main() {
    ln, err := aquitar.Listen(context.Background(), aquitar.Config{
        Server: "tunnel.example.com:8443",
        PSK:    "my-secret-key",
        Port:   9002,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer ln.Close()

    log.Println("Echo server listening on public port 9002")
    for {
        conn, err := ln.Accept()
        if err != nil {
            log.Printf("Accept error: %v", err)
            return
        }

        go func() {
            defer conn.Close()
            io.Copy(conn, conn) // Echo back
        }()
    }
}
```

### Example 3: Graceful Shutdown

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/atelier-bm/aquitar"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    ln, err := aquitar.Listen(ctx, aquitar.Config{
        Server: "tunnel.example.com:8443",
        PSK:    "my-secret-key",
        Port:   9003,
    })
    if err != nil {
        log.Fatal(err)
    }

    server := &http.Server{
        Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Write([]byte("OK\n"))
        }),
    }

    // Handle shutdown signals
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigCh
        log.Println("Shutting down...")

        // Graceful shutdown
        shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer shutdownCancel()

        if err := server.Shutdown(shutdownCtx); err != nil {
            log.Printf("Shutdown error: %v", err)
        }

        ln.Close()
        cancel()
    }()

    log.Println("Server running on public port 9003")
    if err := server.Serve(ln); err != http.ErrServerClosed {
        log.Fatal(err)
    }
}
```

### Example 4: Multiple Services

```go
package main

import (
    "context"
    "log"
    "net/http"
    "sync"

    "github.com/atelier-bm/aquitar"
)

func main() {
    ctx := context.Background()
    var wg sync.WaitGroup

    // Service 1: API on port 9001
    wg.Add(1)
    go func() {
        defer wg.Done()
        ln, err := aquitar.Listen(ctx, aquitar.Config{
            Server: "tunnel.example.com:8443",
            PSK:    "my-secret-key",
            Port:   9001,
        })
        if err != nil {
            log.Fatal(err)
        }
        defer ln.Close()

        http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Write([]byte("API Service\n"))
        }))
    }()

    // Service 2: Admin on port 9002
    wg.Add(1)
    go func() {
        defer wg.Done()
        ln, err := aquitar.Listen(ctx, aquitar.Config{
            Server: "tunnel.example.com:8443",
            PSK:    "my-secret-key",
            Port:   9002,
        })
        if err != nil {
            log.Fatal(err)
        }
        defer ln.Close()

        http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Write([]byte("Admin Service\n"))
        }))
    }()

    log.Println("Multiple services running")
    wg.Wait()
}
```

## Security

### Pre-Shared Key (PSK)

The PSK is your primary authentication mechanism. Treat it like a password:

**Generate a strong PSK:**

```bash
# 256-bit random key (recommended)
openssl rand -hex 32

# Or 128-bit
openssl rand -hex 16
```

**Protect your PSK:**
- Never commit PSKs to version control
- Use environment variables or secret management
- Rotate regularly
- Use different PSKs for different environments

### TLS Configuration

**Production deployments MUST use TLS:**

```bash
aquitar-server -domain tunnel.example.com -psk "$PSK"
```

This enables:
- Automatic Let's Encrypt certificates via `autocert`
- TLS 1.2+ encryption for all connections
- Certificate auto-renewal

**DNS Requirements:**
- Domain must point to your server's IP
- Port 443 must be accessible for ACME HTTP-01 challenge (or use TLS-ALPN-01)

### Network Security

**Firewall recommendations:**

```bash
# Allow control port
ufw allow 8443/tcp

# Allow client ports (adjust as needed)
ufw allow 9000:9100/tcp

# Block everything else
ufw default deny incoming
ufw enable
```

**Docker port exposure:**

Only expose ports you actually need:

```yaml
ports:
  - "8443:8443"      # Control port
  - "9001-9010:9001-9010"  # Limited range
```

### Port Security

**Disallow sensitive ports:**

```json
{
  "disallowed_ports": [
    22,    // SSH
    25,    // SMTP
    80,    // HTTP
    443,   // HTTPS
    3306,  // MySQL
    5432,  // PostgreSQL
    8443   // Control port
  ]
}
```

The control port is always disallowed automatically.

## Troubleshooting

### Common Errors

#### `invalid psk`

**Cause:** PSK mismatch between client and server.

**Solution:**
```bash
# Verify server PSK
echo $AQUITAR_PSK

# Verify client PSK in code
log.Printf("Using PSK: %s", cfg.PSK)
```

#### `port in use`

**Cause:** Another client has already claimed this port.

**Solutions:**
1. Choose a different port
2. Stop the other client
3. Check server logs: `docker logs aquitar`

#### `port disallowed`

**Cause:** Server configuration prevents this port.

**Solution:** 
- Request a different port
- Contact server administrator to adjust `disallowed_ports`

#### `connection refused`

**Cause:** Server not reachable.

**Check:**
```bash
# Test connectivity
nc -zv tunnel.example.com 8443

# Check server is running
docker ps | grep aquitar
```

#### TLS Certificate Issues

**Cause:** Domain not pointing to server or ACME challenge failed.

**Verify:**
```bash
# Check DNS
dig tunnel.example.com

# Check port 443 is accessible
curl -v https://tunnel.example.com
```

**Solution:**
- Ensure domain DNS points to server IP
- Verify port 443 is open
- Check server logs for ACME errors

### Debug Logging

#### Server Logs

```bash
# Docker
docker logs aquitar -f

# Binary
aquitar-server -psk "$PSK" 2>&1 | tee server.log
```

#### Client Debugging

```go
import "log"

ln, err := aquitar.Listen(ctx, cfg)
if err != nil {
    log.Printf("Connection failed: %v", err)
    log.Printf("Server: %s, Port: %d", cfg.Server, cfg.Port)
    return err
}

log.Printf("Connected successfully")
log.Printf("Listener address: %v", ln.Addr())
```

### Health Checks

#### Server Health

```bash
# Check if port is listening
nc -zv localhost 8443

# Docker health status
docker inspect aquitar | jq '.[0].State.Health'
```

#### Connection Test

```bash
# Test from client machine
nc -zv tunnel.example.com 8443
```

## Development

### Requirements

- Go 1.21 or later
- Docker (for container builds)

### Building from Source

```bash
# Clone repository
git clone https://github.com/atelier-bm/aquitar.git
cd aquitar

# Download dependencies
go mod download

# Build server
go build -o aquitar-server ./cmd/aquitar-server

# Run tests
go test -race -cover ./...
```

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Specific package
go test ./server -v

# Race detection
go test -race ./...
```

### Docker Build

```bash
# Build image
docker build -t aquitar:dev .

# Run locally
docker run --rm -p 8443:8443 \
  -e AQUITAR_PSK=test-key \
  aquitar:dev
```

### Project Structure

```
aquitar/
├── cmd/
│   └── aquitar-server/     # Server binary
│       └── main.go
├── server/                 # Server implementation
│   ├── server.go
│   └── session.go
├── aquitar.go              # Public API
├── client.go               # Client implementation
├── protocol.go             # Wire protocol
├── docs/
│   └── SPEC.md            # Technical specification
├── Dockerfile
├── docker-compose.yml
└── README.md
```

### Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make changes and add tests
4. Run tests: `go test ./...`
5. Commit: `git commit -am 'Add feature'`
6. Push: `git push origin feature/my-feature`
7. Open a Pull Request

### Continuous Integration

The project uses GitHub Actions for CI:
- Automated testing on push/PR
- Linting with golangci-lint
- Docker image builds
- Vulnerability scanning with Trivy

See [`.github/workflows/ci.yml`](.github/workflows/ci.yml) for details.

## Architecture

For technical details on the wire protocol, multiplexing, and internal architecture, see [`docs/SPEC.md`](docs/SPEC.md).

## License

[License information to be added]

## Support

- **Issues:** [GitHub Issues](https://github.com/atelier-bm/aquitar/issues)
- **Documentation:** [docs/](docs/)
- **Specification:** [docs/SPEC.md](docs/SPEC.md)

---

**May the tunnel be with you.**
