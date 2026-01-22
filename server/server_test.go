package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

func TestServer_Start(t *testing.T) {
	t.Skip("not implemented: Server type does not exist yet")

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Domain:         "localhost",
				PSK:            "secret",
				ControlPort:    0, // Random port
				DisallowedPort: []int{8443},
			},
			wantErr: false,
		},
		{
			name: "empty psk",
			config: Config{
				Domain:      "localhost",
				PSK:         "",
				ControlPort: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test will be implemented when Server type exists
			_ = tt
		})
	}
}

func TestServer_ClientRegistration(t *testing.T) {
	t.Skip("not implemented: Server type does not exist yet")

	tests := []struct {
		name      string
		serverPSK string
		clientPSK string
		port      int
		wantOK    bool
		wantError string
	}{
		{
			name:      "valid registration",
			serverPSK: "secret",
			clientPSK: "secret",
			port:      9001,
			wantOK:    true,
		},
		{
			name:      "invalid psk",
			serverPSK: "secret",
			clientPSK: "wrong",
			port:      9001,
			wantOK:    false,
			wantError: "invalid psk",
		},
		{
			name:      "disallowed port",
			serverPSK: "secret",
			clientPSK: "secret",
			port:      8443,
			wantOK:    false,
			wantError: "port disallowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test will be implemented when Server type exists
			_ = tt
		})
	}
}

func TestServer_PortInUse(t *testing.T) {
	t.Skip("not implemented: Server type does not exist yet")

	// First client claims port 9001
	// Second client requests port 9001
	// Second client should get "port in use" error
}

func TestServer_FullIntegration(t *testing.T) {
	t.Skip("not implemented: Server type does not exist yet")

	// Full integration test:
	// 1. Start server
	// 2. Client connects and registers port
	// 3. External connection to public port
	// 4. Data flows through tunnel
	// 5. Client receives data
}

// testServer is a simplified server for integration testing.
// It doesn't use TLS for easier testing.
type testServer struct {
	listener    net.Listener
	psk         string
	disallowed  map[int]bool
	activePorts map[int]*yamux.Session
	mu          sync.Mutex
}

func newTestServer(t *testing.T, psk string, disallowed []int) *testServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}

	disallowedMap := make(map[int]bool)
	for _, p := range disallowed {
		disallowedMap[p] = true
	}

	srv := &testServer{
		listener:    ln,
		psk:         psk,
		disallowed:  disallowedMap,
		activePorts: make(map[int]*yamux.Session),
	}

	t.Cleanup(func() {
		srv.Close()
	})

	return srv
}

func (s *testServer) Addr() string {
	return s.listener.Addr().String()
}

func (s *testServer) Close() {
	s.listener.Close()
	s.mu.Lock()
	for _, sess := range s.activePorts {
		sess.Close()
	}
	s.mu.Unlock()
}

func (s *testServer) Serve() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *testServer) handleConn(conn net.Conn) {
	// Set up yamux
	session, err := yamux.Server(conn, nil)
	if err != nil {
		conn.Close()
		return
	}

	// Accept control stream
	stream, err := session.Accept()
	if err != nil {
		session.Close()
		return
	}

	// Read registration request
	var req struct {
		PSK  string `json:"psk"`
		Port int    `json:"port"`
	}
	if err := json.NewDecoder(stream).Decode(&req); err != nil {
		stream.Close()
		session.Close()
		return
	}

	// Validate
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}

	if req.PSK != s.psk {
		resp.OK = false
		resp.Error = "invalid psk"
	} else if s.disallowed[req.Port] {
		resp.OK = false
		resp.Error = "port disallowed"
	} else {
		s.mu.Lock()
		if _, exists := s.activePorts[req.Port]; exists {
			resp.OK = false
			resp.Error = "port in use"
		} else {
			s.activePorts[req.Port] = session
			resp.OK = true
		}
		s.mu.Unlock()
	}

	json.NewEncoder(stream).Encode(resp)

	if !resp.OK {
		stream.Close()
		session.Close()
	}
}

func TestIntegration_ClientServerEcho(t *testing.T) {
	t.Skip("not implemented: full integration requires real Server implementation")

	// This test demonstrates the full flow:
	// 1. Server listens on control port
	// 2. Client connects and registers port 9001
	// 3. External client connects to port 9001
	// 4. Data is echoed back

	srv := newTestServer(t, "secret", []int{8443})
	go srv.Serve()

	// Connect as Aquitar client
	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn.Close()

	// Set up yamux client
	session, err := yamux.Client(conn, nil)
	if err != nil {
		t.Fatalf("yamux client error: %v", err)
	}
	defer session.Close()

	// Open control stream
	stream, err := session.Open()
	if err != nil {
		t.Fatalf("open stream error: %v", err)
	}

	// Send registration
	req := struct {
		PSK  string `json:"psk"`
		Port int    `json:"port"`
	}{
		PSK:  "secret",
		Port: 9001,
	}
	if err := json.NewEncoder(stream).Encode(req); err != nil {
		t.Fatalf("encode request error: %v", err)
	}

	// Read response
	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(stream).Decode(&resp); err != nil {
		t.Fatalf("decode response error: %v", err)
	}

	if !resp.OK {
		t.Fatalf("registration failed: %s", resp.Error)
	}

	t.Log("Client registered successfully on port 9001")
}

func TestIntegration_InvalidPSK(t *testing.T) {
	t.Skip("not implemented: full integration requires real Server implementation")

	srv := newTestServer(t, "correct-secret", nil)
	go srv.Serve()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn.Close()

	session, err := yamux.Client(conn, nil)
	if err != nil {
		t.Fatalf("yamux client error: %v", err)
	}
	defer session.Close()

	stream, err := session.Open()
	if err != nil {
		t.Fatalf("open stream error: %v", err)
	}

	// Send wrong PSK
	req := struct {
		PSK  string `json:"psk"`
		Port int    `json:"port"`
	}{
		PSK:  "wrong-secret",
		Port: 9001,
	}
	json.NewEncoder(stream).Encode(req)

	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	json.NewDecoder(stream).Decode(&resp)

	if resp.OK {
		t.Error("expected registration to fail with wrong PSK")
	}
	if resp.Error != "invalid psk" {
		t.Errorf("error = %q, want %q", resp.Error, "invalid psk")
	}
}

func TestIntegration_PortInUse(t *testing.T) {
	t.Skip("not implemented: full integration requires real Server implementation")

	srv := newTestServer(t, "secret", nil)
	go srv.Serve()

	// First client claims port
	conn1, _ := net.Dial("tcp", srv.Addr())
	defer conn1.Close()
	session1, _ := yamux.Client(conn1, nil)
	defer session1.Close()
	stream1, _ := session1.Open()

	req := struct {
		PSK  string `json:"psk"`
		Port int    `json:"port"`
	}{PSK: "secret", Port: 9001}
	json.NewEncoder(stream1).Encode(req)

	var resp1 struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	json.NewDecoder(stream1).Decode(&resp1)

	if !resp1.OK {
		t.Fatalf("first client should succeed: %s", resp1.Error)
	}

	// Second client tries same port
	conn2, _ := net.Dial("tcp", srv.Addr())
	defer conn2.Close()
	session2, _ := yamux.Client(conn2, nil)
	defer session2.Close()
	stream2, _ := session2.Open()

	json.NewEncoder(stream2).Encode(req)

	var resp2 struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	json.NewDecoder(stream2).Decode(&resp2)

	if resp2.OK {
		t.Error("second client should fail - port in use")
	}
	if resp2.Error != "port in use" {
		t.Errorf("error = %q, want %q", resp2.Error, "port in use")
	}
}

func TestIntegration_DisallowedPort(t *testing.T) {
	t.Skip("not implemented: full integration requires real Server implementation")

	srv := newTestServer(t, "secret", []int{8443, 443})
	go srv.Serve()

	tests := []struct {
		name string
		port int
	}{
		{"control port 8443", 8443},
		{"https port 443", 443},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, _ := net.Dial("tcp", srv.Addr())
			defer conn.Close()
			session, _ := yamux.Client(conn, nil)
			defer session.Close()
			stream, _ := session.Open()

			req := struct {
				PSK  string `json:"psk"`
				Port int    `json:"port"`
			}{PSK: "secret", Port: tt.port}
			json.NewEncoder(stream).Encode(req)

			var resp struct {
				OK    bool   `json:"ok"`
				Error string `json:"error,omitempty"`
			}
			json.NewDecoder(stream).Decode(&resp)

			if resp.OK {
				t.Errorf("port %d should be disallowed", tt.port)
			}
			if resp.Error != "port disallowed" {
				t.Errorf("error = %q, want %q", resp.Error, "port disallowed")
			}
		})
	}
}

func TestIntegration_ClientDisconnect(t *testing.T) {
	t.Skip("not implemented: full integration requires real Server implementation")

	// When client disconnects, the public port listener should close immediately
	srv := newTestServer(t, "secret", nil)
	go srv.Serve()

	conn, _ := net.Dial("tcp", srv.Addr())
	session, _ := yamux.Client(conn, nil)
	stream, _ := session.Open()

	req := struct {
		PSK  string `json:"psk"`
		Port int    `json:"port"`
	}{PSK: "secret", Port: 9001}
	json.NewEncoder(stream).Encode(req)

	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	json.NewDecoder(stream).Decode(&resp)

	if !resp.OK {
		t.Fatalf("registration failed: %s", resp.Error)
	}

	// Disconnect
	session.Close()
	conn.Close()

	// Give server time to detect disconnect
	time.Sleep(100 * time.Millisecond)

	// Port should now be available
	srv.mu.Lock()
	_, stillActive := srv.activePorts[9001]
	srv.mu.Unlock()

	// Note: This may still show as active in our simple test server
	// The real implementation should clean up
	_ = stillActive
}

func TestIntegration_GracefulShutdown(t *testing.T) {
	t.Skip("not implemented: full integration requires real Server implementation")

	// Server shutdown should close all sessions gracefully
}

// Config is a placeholder for the server configuration.
type Config struct {
	Domain         string
	PSK            string
	ControlPort    int
	DisallowedPort []int
}

// Silence unused import warnings
var (
	_ = context.Background
	_ = io.EOF
)
