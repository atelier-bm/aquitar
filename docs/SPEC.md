# Aquitar Specification

## Overview

Aquitar is a reverse tunnel proxy that exposes internal services to the internet. Clients connect outbound to the proxy server and request a public port. The proxy listens on that port and forwards traffic back through the tunnel.

The Go client module provides a `net.Listener` interface, so applications use it exactly like a local listener.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│              Hetzner (aquitar-server)               │
│                                                     │
│  :8443 (TLS) ──► Control + multiplexed data        │
│                                                     │
│  :443  (TCP) ──► Forward to client A               │
│  :9001 (TCP) ──► Forward to client B (rattler)     │
└─────────────────────────────────────────────────────┘
        ▲
        │ TLS + yamux multiplexing
        │
┌─────────────────────────────────────────────────────┐
│              Mac mini (aquitar client)              │
│                                                     │
│  aquitar.Listen() → net.Listener                   │
│       │                                             │
│       └──► http.Serve(ln, handler)                 │
└─────────────────────────────────────────────────────┘
```

## Client API

```go
package aquitar

type Config struct {
    Server string // "tunnel.example.com:8443"
    PSK    string // Pre-shared key
    Port   int    // Requested public port
}

// Listen connects to the proxy and returns a net.Listener
func Listen(ctx context.Context, cfg Config) (net.Listener, error)
```

## Server Configuration

```json
{
  "domain": "tunnel.example.com",
  "psk": "your-secret-here"
}
```

Defaults:
- Control port: 8443
- Disallowed ports: [8443]
- Cert cache: `~/.cache/aquitar/certs`

## Wire Protocol

TLS connection with yamux multiplexing.

**Stream 0 (control):**

Client sends:
```json
{"psk": "secret", "port": 9001}
```

Server responds:
```json
{"ok": true}
```

Or on error:
```json
{"ok": false, "error": "port in use"}
```

**Subsequent streams:** Each incoming connection on the public port opens a new yamux stream to the client.

## TLS

Server uses `autocert` for automatic Let's Encrypt certificates. Requires:
- Domain pointing to server
- Port 443 accessible (for ACME HTTP-01 challenge) OR use TLS-ALPN-01 on 8443

## Error Conditions

| Error | Condition |
|-------|-----------|
| `invalid psk` | PSK doesn't match server config |
| `port in use` | Another client has claimed that port |
| `port disallowed` | Port is in disallowed list (e.g., 8443) |

## Lifecycle

1. Client connects to server:8443 over TLS
2. Client sends RegisterRequest with PSK and desired port
3. Server validates PSK, checks port availability
4. Server starts listening on public port
5. Server responds with success
6. Client's `Listen()` returns `net.Listener`
7. Each connection on public port → new yamux stream → `Accept()` returns `net.Conn`
8. On client disconnect, server stops listening on public port immediately

## File Structure

```
aquitar/
├── cmd/
│   └── aquitar-server/
│       └── main.go
├── aquitar.go          # Public API
├── client.go           # Client implementation
├── protocol.go         # Wire protocol types
├── server/
│   ├── server.go       # Server core
│   └── session.go      # Per-client session
├── go.mod
├── docs/
│   └── SPEC.md
└── README.md
```

## Dependencies

- `github.com/hashicorp/yamux` - Connection multiplexing
- `golang.org/x/crypto/acme/autocert` - Let's Encrypt
