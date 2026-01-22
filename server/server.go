// Package server implements the Aquitar proxy server.
package server

import (
	"context"
	"errors"
	"net"
)

// Server is the Aquitar proxy server.
type Server struct {
	config   ServerConfig
	listener net.Listener
}

// ServerConfig holds the server configuration.
type ServerConfig struct {
	// Domain is the server's domain name (for TLS certificates)
	Domain string

	// PSK is the pre-shared key for client authentication
	PSK string

	// ControlPort is the port for client connections (default: 8443)
	ControlPort int

	// DisallowedPorts are ports that clients cannot request
	DisallowedPorts []int

	// CertCacheDir is the directory for caching TLS certificates
	CertCacheDir string
}

// New creates a new Aquitar server.
func New(cfg ServerConfig) (*Server, error) {
	// TODO: Implement
	return nil, errors.New("not implemented")
}

// ListenAndServe starts the server.
func (s *Server) ListenAndServe(ctx context.Context) error {
	// TODO: Implement
	return errors.New("not implemented")
}

// Close shuts down the server gracefully.
func (s *Server) Close() error {
	// TODO: Implement
	return nil
}
