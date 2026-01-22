// Package server implements the Aquitar proxy server.
package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	aquitar "github.com/atelier-bm/aquitar"
	"github.com/hashicorp/yamux"
	"golang.org/x/crypto/acme/autocert"
)

// Server is the Aquitar proxy server.
type Server struct {
	config   ServerConfig
	listener net.Listener

	mu              sync.Mutex
	sessions        map[int]*Session // port -> session
	disallowedPorts map[int]bool
	closed          bool
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

	// Listener allows providing a custom listener (for testing without TLS)
	Listener net.Listener
}

// New creates a new Aquitar server.
func New(cfg ServerConfig) (*Server, error) {
	if cfg.PSK == "" {
		return nil, errors.New("PSK required")
	}

	// Set defaults
	if cfg.ControlPort == 0 {
		cfg.ControlPort = 8443
	}
	if cfg.CertCacheDir == "" {
		cfg.CertCacheDir = "~/.cache/aquitar/certs"
	}

	// Build disallowed ports map
	disallowed := make(map[int]bool)
	for _, p := range cfg.DisallowedPorts {
		disallowed[p] = true
	}
	// Control port is always disallowed
	disallowed[cfg.ControlPort] = true

	return &Server{
		config:          cfg,
		sessions:        make(map[int]*Session),
		disallowedPorts: disallowed,
	}, nil
}

// ListenAndServe starts the server.
func (s *Server) ListenAndServe(ctx context.Context) error {
	var err error

	if s.config.Listener != nil {
		// Use provided listener (for testing)
		s.listener = s.config.Listener
	} else if s.config.Domain != "" && s.config.Domain != "localhost" {
		// Use TLS with autocert for production
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(s.config.Domain),
			Cache:      autocert.DirCache(s.config.CertCacheDir),
		}
		tlsCfg := m.TLSConfig()
		s.listener, err = tls.Listen("tcp", fmt.Sprintf(":%d", s.config.ControlPort), tlsCfg)
		if err != nil {
			return fmt.Errorf("creating TLS listener: %w", err)
		}
	} else {
		// Plain TCP for development/testing
		s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.config.ControlPort))
		if err != nil {
			return fmt.Errorf("creating listener: %w", err)
		}
	}

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		s.Close()
	}()

	// Accept connections
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return nil
			}
			return fmt.Errorf("accepting connection: %w", err)
		}
		go s.handleConn(conn)
	}
}

// handleConn handles a new client connection.
func (s *Server) handleConn(conn net.Conn) {
	// Set up yamux server session
	session, err := yamux.Server(conn, nil)
	if err != nil {
		log.Printf("yamux server error: %v", err)
		conn.Close()
		return
	}

	// Accept control stream (stream 0)
	ctrl, err := session.Accept()
	if err != nil {
		log.Printf("accept control stream error: %v", err)
		session.Close()
		return
	}

	// Read registration request
	var req aquitar.RegisterRequest
	if err := json.NewDecoder(ctrl).Decode(&req); err != nil {
		log.Printf("decode request error: %v", err)
		ctrl.Close()
		session.Close()
		return
	}

	// Validate PSK
	if req.PSK != s.config.PSK {
		s.sendError(ctrl, "invalid psk")
		ctrl.Close()
		session.Close()
		return
	}

	// Check disallowed ports
	if s.disallowedPorts[req.Port] {
		s.sendError(ctrl, "port disallowed")
		ctrl.Close()
		session.Close()
		return
	}

	// Try to claim the port
	s.mu.Lock()
	if _, exists := s.sessions[req.Port]; exists {
		s.mu.Unlock()
		s.sendError(ctrl, "port in use")
		ctrl.Close()
		session.Close()
		return
	}

	// Create session and start public port listener
	sess := NewSession(session, req.Port)
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", req.Port))
	if err != nil {
		s.mu.Unlock()
		s.sendError(ctrl, fmt.Sprintf("failed to listen on port: %v", err))
		ctrl.Close()
		session.Close()
		return
	}
	sess.listener = ln
	s.sessions[req.Port] = sess
	s.mu.Unlock()

	// Send success response
	resp := aquitar.RegisterResponse{OK: true}
	if err := json.NewEncoder(ctrl).Encode(resp); err != nil {
		log.Printf("encode response error: %v", err)
		s.removeSession(req.Port)
		sess.Close()
		return
	}

	// Start proxying connections
	go s.proxySession(sess)

	// Wait for session to close (control stream closes or yamux dies)
	// When that happens, clean up
	<-session.CloseChan()
	s.removeSession(req.Port)
}

// proxySession accepts connections on the public port and proxies them to the client.
func (s *Server) proxySession(sess *Session) {
	for {
		conn, err := sess.listener.Accept()
		if err != nil {
			// Listener closed
			return
		}

		// Open a new stream to the client
		stream, err := sess.yamux.Open()
		if err != nil {
			log.Printf("failed to open stream to client: %v", err)
			conn.Close()
			sess.Close()
			return
		}

		// Proxy in background
		go proxy(conn, stream)
	}
}

// sendError sends an error response on the control stream.
func (s *Server) sendError(ctrl net.Conn, msg string) {
	resp := aquitar.RegisterResponse{OK: false, Error: msg}
	json.NewEncoder(ctrl).Encode(resp)
}

// removeSession removes a session from the active sessions map.
func (s *Server) removeSession(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, port)
}

// Close shuts down the server gracefully.
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.mu.Unlock()

	// Close all sessions
	for _, sess := range sessions {
		sess.Close()
	}

	// Close main listener
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Addr returns the server's listening address.
func (s *Server) Addr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}
