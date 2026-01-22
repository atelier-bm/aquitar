package aquitar

import (
	"encoding/json"
	"testing"
)

func TestRegisterRequest_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		req  RegisterRequest
	}{
		{
			name: "basic request",
			req: RegisterRequest{
				PSK:  "secret-key",
				Port: 9001,
			},
		},
		{
			name: "empty psk",
			req: RegisterRequest{
				PSK:  "",
				Port: 443,
			},
		},
		{
			name: "high port number",
			req: RegisterRequest{
				PSK:  "test",
				Port: 65535,
			},
		},
		{
			name: "low port number",
			req: RegisterRequest{
				PSK:  "test",
				Port: 1,
			},
		},
		{
			name: "zero port",
			req: RegisterRequest{
				PSK:  "test",
				Port: 0,
			},
		},
		{
			name: "psk with special characters",
			req: RegisterRequest{
				PSK:  `secret"with\special/chars`,
				Port: 8080,
			},
		},
		{
			name: "unicode psk",
			req: RegisterRequest{
				PSK:  "秘密キー",
				Port: 9000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// Unmarshal
			var got RegisterRequest
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			// Compare
			if got.PSK != tt.req.PSK {
				t.Errorf("PSK = %q, want %q", got.PSK, tt.req.PSK)
			}
			if got.Port != tt.req.Port {
				t.Errorf("Port = %d, want %d", got.Port, tt.req.Port)
			}
		})
	}
}

func TestRegisterRequest_JSONFormat(t *testing.T) {
	tests := []struct {
		name     string
		req      RegisterRequest
		wantJSON string
	}{
		{
			name: "matches wire protocol spec",
			req: RegisterRequest{
				PSK:  "secret",
				Port: 9001,
			},
			wantJSON: `{"psk":"secret","port":9001}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			if string(data) != tt.wantJSON {
				t.Errorf("JSON = %s, want %s", data, tt.wantJSON)
			}
		})
	}
}

func TestRegisterResponse_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		resp RegisterResponse
	}{
		{
			name: "success response",
			resp: RegisterResponse{
				OK:    true,
				Error: "",
			},
		},
		{
			name: "error invalid psk",
			resp: RegisterResponse{
				OK:    false,
				Error: "invalid psk",
			},
		},
		{
			name: "error port in use",
			resp: RegisterResponse{
				OK:    false,
				Error: "port in use",
			},
		},
		{
			name: "error port disallowed",
			resp: RegisterResponse{
				OK:    false,
				Error: "port disallowed",
			},
		},
		{
			name: "custom error message",
			resp: RegisterResponse{
				OK:    false,
				Error: "unexpected error: connection reset",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// Unmarshal
			var got RegisterResponse
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			// Compare
			if got.OK != tt.resp.OK {
				t.Errorf("OK = %v, want %v", got.OK, tt.resp.OK)
			}
			if got.Error != tt.resp.Error {
				t.Errorf("Error = %q, want %q", got.Error, tt.resp.Error)
			}
		})
	}
}

func TestRegisterResponse_JSONFormat(t *testing.T) {
	tests := []struct {
		name     string
		resp     RegisterResponse
		wantJSON string
	}{
		{
			name: "success matches wire protocol",
			resp: RegisterResponse{
				OK: true,
			},
			wantJSON: `{"ok":true}`,
		},
		{
			name: "error matches wire protocol",
			resp: RegisterResponse{
				OK:    false,
				Error: "port in use",
			},
			wantJSON: `{"ok":false,"error":"port in use"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			if string(data) != tt.wantJSON {
				t.Errorf("JSON = %s, want %s", data, tt.wantJSON)
			}
		})
	}
}

func TestRegisterRequest_UnmarshalFromWire(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		wantPSK  string
		wantPort int
		wantErr  bool
	}{
		{
			name:     "valid wire format",
			jsonData: `{"psk": "secret", "port": 9001}`,
			wantPSK:  "secret",
			wantPort: 9001,
			wantErr:  false,
		},
		{
			name:     "no spaces",
			jsonData: `{"psk":"secret","port":9001}`,
			wantPSK:  "secret",
			wantPort: 9001,
			wantErr:  false,
		},
		{
			name:     "extra whitespace",
			jsonData: `{  "psk" :  "secret" , "port" :  9001  }`,
			wantPSK:  "secret",
			wantPort: 9001,
			wantErr:  false,
		},
		{
			name:     "missing psk field",
			jsonData: `{"port": 9001}`,
			wantPSK:  "",
			wantPort: 9001,
			wantErr:  false,
		},
		{
			name:     "missing port field",
			jsonData: `{"psk": "secret"}`,
			wantPSK:  "secret",
			wantPort: 0,
			wantErr:  false,
		},
		{
			name:     "invalid json",
			jsonData: `{invalid}`,
			wantErr:  true,
		},
		{
			name:     "empty object",
			jsonData: `{}`,
			wantPSK:  "",
			wantPort: 0,
			wantErr:  false,
		},
		{
			name:     "wrong type for port",
			jsonData: `{"psk": "secret", "port": "9001"}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req RegisterRequest
			err := json.Unmarshal([]byte(tt.jsonData), &req)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if req.PSK != tt.wantPSK {
				t.Errorf("PSK = %q, want %q", req.PSK, tt.wantPSK)
			}
			if req.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", req.Port, tt.wantPort)
			}
		})
	}
}

func TestRegisterResponse_UnmarshalFromWire(t *testing.T) {
	tests := []struct {
		name      string
		jsonData  string
		wantOK    bool
		wantError string
		wantErr   bool
	}{
		{
			name:     "success response",
			jsonData: `{"ok": true}`,
			wantOK:   true,
			wantErr:  false,
		},
		{
			name:      "error response",
			jsonData:  `{"ok": false, "error": "port in use"}`,
			wantOK:    false,
			wantError: "port in use",
			wantErr:   false,
		},
		{
			name:      "invalid psk error",
			jsonData:  `{"ok": false, "error": "invalid psk"}`,
			wantOK:    false,
			wantError: "invalid psk",
			wantErr:   false,
		},
		{
			name:      "port disallowed error",
			jsonData:  `{"ok": false, "error": "port disallowed"}`,
			wantOK:    false,
			wantError: "port disallowed",
			wantErr:   false,
		},
		{
			name:     "invalid json",
			jsonData: `{broken`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp RegisterResponse
			err := json.Unmarshal([]byte(tt.jsonData), &resp)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if resp.OK != tt.wantOK {
				t.Errorf("OK = %v, want %v", resp.OK, tt.wantOK)
			}
			if resp.Error != tt.wantError {
				t.Errorf("Error = %q, want %q", resp.Error, tt.wantError)
			}
		})
	}
}
