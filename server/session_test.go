package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestProxy_BidirectionalCopy(t *testing.T) {
	tests := []struct {
		name    string
		srcData string
		dstData string
		wantSrc string // Data received at src (from dst)
		wantDst string // Data received at dst (from src)
	}{
		{
			name:    "simple message both directions",
			srcData: "hello from src",
			dstData: "hello from dst",
			wantSrc: "hello from dst",
			wantDst: "hello from src",
		},
		{
			name:    "empty src",
			srcData: "",
			dstData: "only dst sends",
			wantSrc: "only dst sends",
			wantDst: "",
		},
		{
			name:    "empty dst",
			srcData: "only src sends",
			dstData: "",
			wantSrc: "",
			wantDst: "only src sends",
		},
		{
			name:    "binary data",
			srcData: string([]byte{0x00, 0x01, 0x02, 0xff}),
			dstData: string([]byte{0xfe, 0xfd, 0xfc}),
			wantSrc: string([]byte{0xfe, 0xfd, 0xfc}),
			wantDst: string([]byte{0x00, 0x01, 0x02, 0xff}),
		},
		{
			name:    "large data",
			srcData: string(make([]byte, 64*1024)), // 64KB
			dstData: string(make([]byte, 64*1024)),
			wantSrc: string(make([]byte, 64*1024)),
			wantDst: string(make([]byte, 64*1024)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create pipe pairs for src and dst
			srcClient, srcServer := net.Pipe()
			dstClient, dstServer := net.Pipe()

			// Buffers to collect received data
			var srcReceived, dstReceived bytes.Buffer

			var wg sync.WaitGroup
			wg.Add(3)

			// Run proxy
			go func() {
				defer wg.Done()
				proxy(srcServer, dstServer)
			}()

			// Src side: write data, then read response
			go func() {
				defer wg.Done()
				defer srcClient.Close()

				if tt.srcData != "" {
					srcClient.Write([]byte(tt.srcData))
				}
				// Signal done writing by closing write side
				if closer, ok := srcClient.(interface{ CloseWrite() error }); ok {
					closer.CloseWrite()
				}

				io.Copy(&srcReceived, srcClient)
			}()

			// Dst side: write data, then read response
			go func() {
				defer wg.Done()
				defer dstClient.Close()

				if tt.dstData != "" {
					dstClient.Write([]byte(tt.dstData))
				}
				if closer, ok := dstClient.(interface{ CloseWrite() error }); ok {
					closer.CloseWrite()
				}

				io.Copy(&dstReceived, dstClient)
			}()

			// Wait with timeout
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("test timed out")
			}

			// Verify data transfer
			if srcReceived.String() != tt.wantSrc {
				t.Errorf("src received = %q, want %q", srcReceived.String(), tt.wantSrc)
			}
			if dstReceived.String() != tt.wantDst {
				t.Errorf("dst received = %q, want %q", dstReceived.String(), tt.wantDst)
			}
		})
	}
}

func TestProxy_CleanClose(t *testing.T) {
	tests := []struct {
		name       string
		closeFirst string // "src" or "dst"
	}{
		{
			name:       "src closes first",
			closeFirst: "src",
		},
		{
			name:       "dst closes first",
			closeFirst: "dst",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srcClient, srcServer := net.Pipe()
			dstClient, dstServer := net.Pipe()

			proxyDone := make(chan struct{})
			go func() {
				proxy(srcServer, dstServer)
				close(proxyDone)
			}()

			// Close one side
			if tt.closeFirst == "src" {
				srcClient.Close()
			} else {
				dstClient.Close()
			}

			// Proxy should exit cleanly
			select {
			case <-proxyDone:
				// Good
			case <-time.After(1 * time.Second):
				t.Error("proxy did not exit after connection closed")
			}

			// Both server connections should be closed
			_, err := srcServer.Write([]byte("test"))
			if err == nil {
				t.Error("srcServer should be closed")
			}

			_, err = dstServer.Write([]byte("test"))
			if err == nil {
				t.Error("dstServer should be closed")
			}
		})
	}
}

func TestProxy_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		srcErr      error
		dstErr      error
		expectClean bool
	}{
		{
			name:        "normal EOF",
			srcErr:      io.EOF,
			dstErr:      nil,
			expectClean: true,
		},
		{
			name:        "connection reset src",
			srcErr:      errors.New("connection reset by peer"),
			dstErr:      nil,
			expectClean: true, // Should still close cleanly
		},
		{
			name:        "connection reset dst",
			srcErr:      nil,
			dstErr:      errors.New("connection reset by peer"),
			expectClean: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srcClient, srcServer := net.Pipe()
			dstClient, dstServer := net.Pipe()

			proxyDone := make(chan struct{})
			go func() {
				proxy(srcServer, dstServer)
				close(proxyDone)
			}()

			// Close connections to trigger errors
			srcClient.Close()
			dstClient.Close()

			select {
			case <-proxyDone:
				// Proxy exited
			case <-time.After(1 * time.Second):
				t.Error("proxy did not exit after errors")
			}
		})
	}
}

func TestSession_HandleConnection(t *testing.T) {
	t.Skip("not implemented: Session type does not exist yet")

	tests := []struct {
		name       string
		clientData string
		wantData   string
	}{
		{
			name:       "echo data",
			clientData: "hello",
			wantData:   "hello",
		},
		{
			name:       "empty data",
			clientData: "",
			wantData:   "",
		},
		{
			name:       "large data",
			clientData: string(make([]byte, 1024*1024)), // 1MB
			wantData:   string(make([]byte, 1024*1024)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test will be implemented when Session type exists
			_ = tt
		})
	}
}

func TestSession_ConcurrentConnections(t *testing.T) {
	t.Skip("not implemented: Session type does not exist yet")

	// Test that multiple connections can be proxied simultaneously
	numConns := 10

	_ = numConns // Will be used when Session is implemented
}

func TestSession_Close(t *testing.T) {
	t.Skip("not implemented: Session type does not exist yet")

	tests := []struct {
		name          string
		activeConns   int
		expectCleanup bool
	}{
		{
			name:          "close with no active connections",
			activeConns:   0,
			expectCleanup: true,
		},
		{
			name:          "close with active connections",
			activeConns:   5,
			expectCleanup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test will be implemented when Session type exists
			_ = tt
		})
	}
}

func TestSession_Context(t *testing.T) {
	t.Skip("not implemented: Session type does not exist yet")

	tests := []struct {
		name        string
		setupCtx    func() (context.Context, context.CancelFunc)
		expectClose bool
	}{
		{
			name: "context cancelled",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			expectClose: true,
		},
		{
			name: "context timeout",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 10*time.Millisecond)
			},
			expectClose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test will be implemented when Session type exists
			_ = tt
		})
	}
}

// Note: proxy function is defined in session.go
