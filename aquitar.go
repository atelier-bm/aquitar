// Package aquitar provides a reverse tunnel proxy client.
//
// The client connects to the proxy server, requests a public port, and returns
// a net.Listener that receives connections forwarded from the public port.
package aquitar

import (
	"context"
	"net"
)

// Config holds the configuration for connecting to an Aquitar proxy server.
type Config struct {
	// Server is the address of the proxy server (e.g., "tunnel.example.com:8443")
	Server string

	// PSK is the pre-shared key for authentication
	PSK string

	// Port is the public port to request from the server
	Port int
}

// Listen connects to the Aquitar proxy server and returns a net.Listener.
// Incoming connections on the requested public port will be forwarded through
// the tunnel and returned by Accept().
//
// The context controls the connection lifetime. If the context is cancelled,
// the listener will be closed.
func Listen(ctx context.Context, cfg Config) (net.Listener, error) {
	return connect(ctx, cfg)
}
