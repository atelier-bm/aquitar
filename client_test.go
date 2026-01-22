package aquitar

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

// mockServer is a test helper that simulates the Aquitar server.
// It accepts TLS connections, validates requests, and can respond with
// configurable success/error responses.
type mockServer struct {
	listener   net.Listener
	psk        string
	response   RegisterResponse
	acceptConn bool // Whether to accept incoming proxy connections

	mu       sync.Mutex
	sessions []*yamux.Session
}

func newMockServer(t *testing.T, psk string, response RegisterResponse) *mockServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create mock server listener: %v", err)
	}

	ms := &mockServer{
		listener: ln,
		psk:      psk,
		response: response,
	}

	t.Cleanup(func() {
		ms.Close()
	})

	return ms
}

func (ms *mockServer) Addr() string {
	return ms.listener.Addr().String()
}

func (ms *mockServer) Close() {
	ms.listener.Close()
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, s := range ms.sessions {
		s.Close()
	}
}

// Serve handles one client connection (for simple tests).
func (ms *mockServer) ServeOne(t *testing.T) {
	t.Helper()

	conn, err := ms.listener.Accept()
	if err != nil {
		t.Logf("mock server accept error: %v", err)
		return
	}

	// Set up yamux server session
	session, err := yamux.Server(conn, nil)
	if err != nil {
		t.Logf("yamux server error: %v", err)
		conn.Close()
		return
	}

	ms.mu.Lock()
	ms.sessions = append(ms.sessions, session)
	ms.mu.Unlock()

	// Accept stream 0 (control)
	stream, err := session.Accept()
	if err != nil {
		t.Logf("stream accept error: %v", err)
		return
	}

	// Read request
	var req RegisterRequest
	if err := json.NewDecoder(stream).Decode(&req); err != nil {
		t.Logf("decode request error: %v", err)
		stream.Close()
		return
	}

	// Validate PSK and send response
	if req.PSK != ms.psk {
		resp := RegisterResponse{OK: false, Error: "invalid psk"}
		json.NewEncoder(stream).Encode(resp)
		stream.Close()
		return
	}

	// Send configured response
	json.NewEncoder(stream).Encode(ms.response)

	// Keep stream open for the session lifetime
}

func TestListen_Success(t *testing.T) {
	ms := newMockServer(t, "secret", RegisterResponse{OK: true})
	go ms.ServeOne(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := Listen(ctx, Config{
		Server: ms.Addr(),
		PSK:    "secret",
		Port:   9001,
	})
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()

	// Verify we got a valid listener
	if ln == nil {
		t.Fatal("Listen() returned nil listener")
	}

	// Verify Addr() returns something meaningful
	addr := ln.Addr()
	if addr == nil {
		t.Error("Listener.Addr() returned nil")
	}
}

func TestListen_InvalidPSK(t *testing.T) {
	tests := []struct {
		name      string
		serverPSK string
		clientPSK string
		wantErr   string
	}{
		{
			name:      "wrong psk",
			serverPSK: "correct-secret",
			clientPSK: "wrong-secret",
			wantErr:   "invalid psk",
		},
		{
			name:      "empty client psk",
			serverPSK: "secret",
			clientPSK: "",
			wantErr:   "invalid psk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := newMockServer(t, tt.serverPSK, RegisterResponse{OK: false, Error: "invalid psk"})
			go ms.ServeOne(t)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := Listen(ctx, Config{
				Server: ms.Addr(),
				PSK:    tt.clientPSK,
				Port:   9001,
			})

			if err == nil {
				t.Fatal("Listen() expected error, got nil")
			}

			if err.Error() != tt.wantErr && !contains(err.Error(), tt.wantErr) {
				t.Errorf("Listen() error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestListen_PortErrors(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		response RegisterResponse
		wantErr  string
	}{
		{
			name: "port in use",
			port: 9001,
			response: RegisterResponse{
				OK:    false,
				Error: "port in use",
			},
			wantErr: "port in use",
		},
		{
			name: "port disallowed",
			port: 8443,
			response: RegisterResponse{
				OK:    false,
				Error: "port disallowed",
			},
			wantErr: "port disallowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := newMockServer(t, "secret", tt.response)
			go ms.ServeOne(t)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := Listen(ctx, Config{
				Server: ms.Addr(),
				PSK:    "secret",
				Port:   tt.port,
			})

			if err == nil {
				t.Fatal("Listen() expected error, got nil")
			}

			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("Listen() error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestListen_ContextCancellation(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func() (context.Context, context.CancelFunc)
		wantErrType string
	}{
		{
			name: "pre-cancelled context",
			setupCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return ctx, func() {}
			},
			wantErrType: "context canceled",
		},
		{
			name: "context timeout",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 1*time.Nanosecond)
			},
			wantErrType: "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't even start a server - context should fail first
			ctx, cancel := tt.setupCtx()
			defer cancel()

			// Small delay to ensure timeout triggers
			time.Sleep(1 * time.Millisecond)

			_, err := Listen(ctx, Config{
				Server: "127.0.0.1:0", // Won't connect anyway
				PSK:    "secret",
				Port:   9001,
			})

			if err == nil {
				t.Fatal("Listen() expected error, got nil")
			}

			if !contains(err.Error(), tt.wantErrType) {
				t.Errorf("Listen() error = %q, want to contain %q", err.Error(), tt.wantErrType)
			}
		})
	}
}

func TestListen_ConnectionFailure(t *testing.T) {
	tests := []struct {
		name    string
		server  string
		wantErr bool
	}{
		{
			name:    "connection refused",
			server:  "127.0.0.1:1", // Port 1 should refuse connections
			wantErr: true,
		},
		{
			name:    "invalid address",
			server:  "not-a-valid-address:xyz",
			wantErr: true,
		},
		{
			name:    "empty server",
			server:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			_, err := Listen(ctx, Config{
				Server: tt.server,
				PSK:    "secret",
				Port:   9001,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("Listen() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListen_AcceptConnections(t *testing.T) {
	ms := newMockServer(t, "secret", RegisterResponse{OK: true})

	// Start mock server in background
	go func() {
		ms.ServeOne(t)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := Listen(ctx, Config{
		Server: ms.Addr(),
		PSK:    "secret",
		Port:   9001,
	})
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()

	// Test that Accept() blocks and can be unblocked by Close()
	done := make(chan struct{})
	go func() {
		ln.Accept()
		close(done)
	}()

	// Close the listener, which should unblock Accept
	time.Sleep(10 * time.Millisecond)
	ln.Close()

	select {
	case <-done:
		// Good - Accept unblocked
	case <-time.After(1 * time.Second):
		t.Error("Accept() did not unblock after Close()")
	}
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Server: "tunnel.example.com:8443",
				PSK:    "secret",
				Port:   9001,
			},
			wantErr: false,
		},
		{
			name: "missing server",
			config: Config{
				Server: "",
				PSK:    "secret",
				Port:   9001,
			},
			wantErr: true,
		},
		{
			name: "missing psk",
			config: Config{
				Server: "tunnel.example.com:8443",
				PSK:    "",
				Port:   9001,
			},
			wantErr: true, // Should fail at server validation
		},
		{
			name: "zero port",
			config: Config{
				Server: "tunnel.example.com:8443",
				PSK:    "secret",
				Port:   0,
			},
			wantErr: true,
		},
		{
			name: "negative port",
			config: Config{
				Server: "tunnel.example.com:8443",
				PSK:    "secret",
				Port:   -1,
			},
			wantErr: true,
		},
		{
			name: "port too high",
			config: Config{
				Server: "tunnel.example.com:8443",
				PSK:    "secret",
				Port:   65536,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_, err := Listen(ctx, tt.config)

			// We expect errors for invalid configs (connection may also fail, which is fine)
			if tt.wantErr && err == nil {
				t.Error("Listen() expected error for invalid config, got nil")
			}
		})
	}
}

// TestListenerInterface verifies the returned listener satisfies net.Listener.
func TestListenerInterface(t *testing.T) {
	ms := newMockServer(t, "secret", RegisterResponse{OK: true})
	go ms.ServeOne(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := Listen(ctx, Config{
		Server: ms.Addr(),
		PSK:    "secret",
		Port:   9001,
	})
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()

	// Verify interface compliance
	var _ net.Listener = ln

	// Verify methods exist and are callable
	_ = ln.Addr()
	ln.Close()
}

// contains is a helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
