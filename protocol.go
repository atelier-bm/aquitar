package aquitar

// RegisterRequest is sent by the client on stream 0 to request a port.
// Wire format: {"psk": "secret", "port": 9001}
type RegisterRequest struct {
	PSK  string `json:"psk"`
	Port int    `json:"port"`
}

// RegisterResponse is sent by the server in reply to a RegisterRequest.
// Wire format (success): {"ok": true}
// Wire format (error): {"ok": false, "error": "port in use"}
type RegisterResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}
