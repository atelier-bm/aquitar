package aquitar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/hashicorp/yamux"
)

// listener implements net.Listener for the Aquitar client.
// It wraps a yamux session and accepts new streams as incoming connections.
type listener struct {
	session *yamux.Session
	ctrl    net.Conn // control stream (stream 0)
	addr    tunnelAddr
}

// tunnelAddr implements net.Addr for the tunnel.
type tunnelAddr struct {
	server string
	port   int
}

func (a tunnelAddr) Network() string {
	return "tcp"
}

func (a tunnelAddr) String() string {
	return fmt.Sprintf("%s (port %d)", a.server, a.port)
}

// Accept waits for and returns the next connection from the tunnel.
func (l *listener) Accept() (net.Conn, error) {
	stream, err := l.session.Accept()
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// Close closes the listener and the underlying connection.
func (l *listener) Close() error {
	// Close control stream first
	if l.ctrl != nil {
		l.ctrl.Close()
	}
	// Then close the session
	if l.session != nil {
		return l.session.Close()
	}
	return nil
}

// Addr returns the listener's network address.
func (l *listener) Addr() net.Addr {
	return l.addr
}

// connect establishes a connection to the server and performs registration.
func connect(ctx context.Context, cfg Config) (*listener, error) {
	// Validate config
	if cfg.Server == "" {
		return nil, errors.New("server address required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, errors.New("invalid port")
	}

	// Check context before dialing
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Dial with context
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", cfg.Server)
	if err != nil {
		return nil, fmt.Errorf("connecting to server: %w", err)
	}

	// Set up yamux client session
	session, err := yamux.Client(conn, nil)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("creating yamux session: %w", err)
	}

	// Open control stream (stream 0)
	ctrl, err := session.Open()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("opening control stream: %w", err)
	}

	// Send registration request
	req := RegisterRequest{
		PSK:  cfg.PSK,
		Port: cfg.Port,
	}
	if err := json.NewEncoder(ctrl).Encode(req); err != nil {
		ctrl.Close()
		session.Close()
		return nil, fmt.Errorf("sending register request: %w", err)
	}

	// Read response
	var resp RegisterResponse
	if err := json.NewDecoder(ctrl).Decode(&resp); err != nil {
		ctrl.Close()
		session.Close()
		return nil, fmt.Errorf("reading register response: %w", err)
	}

	// Check for errors
	if !resp.OK {
		ctrl.Close()
		session.Close()
		return nil, errors.New(resp.Error)
	}

	return &listener{
		session: session,
		ctrl:    ctrl,
		addr: tunnelAddr{
			server: cfg.Server,
			port:   cfg.Port,
		},
	}, nil
}
