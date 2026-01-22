// Package server implements the Aquitar proxy server.
package server

import (
	"io"
	"net"
	"sync"

	"github.com/hashicorp/yamux"
)

// Session represents a connected client session.
type Session struct {
	yamux    *yamux.Session
	port     int
	listener net.Listener

	mu     sync.Mutex
	closed bool
}

// NewSession creates a new client session.
func NewSession(yamuxSession *yamux.Session, port int) *Session {
	return &Session{
		yamux: yamuxSession,
		port:  port,
	}
}

// Port returns the public port assigned to this session.
func (s *Session) Port() int {
	return s.port
}

// Close closes the session and releases the public port.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if s.listener != nil {
		s.listener.Close()
	}
	if s.yamux != nil {
		s.yamux.Close()
	}
	return nil
}

// proxy copies data bidirectionally between two connections.
// It closes both connections when either direction encounters an error.
func proxy(src, dst net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// src -> dst
	go func() {
		defer wg.Done()
		io.Copy(dst, src)
		// Close write side if possible
		if closer, ok := dst.(interface{ CloseWrite() error }); ok {
			closer.CloseWrite()
		}
	}()

	// dst -> src
	go func() {
		defer wg.Done()
		io.Copy(src, dst)
		if closer, ok := src.(interface{ CloseWrite() error }); ok {
			closer.CloseWrite()
		}
	}()

	wg.Wait()

	src.Close()
	dst.Close()
}
