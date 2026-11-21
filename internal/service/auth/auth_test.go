package auth

import (
	"testing"
)

func TestParseExtConfig(t *testing.T) {
	tests := []struct {
		name      string
		extData   string
		wantProto string
		wantHost  string
		wantPort  int
		wantCID   string
		wantErr   bool
	}{
		{
			name:      "empty string returns defaults",
			extData:   "",
			wantProto: "http",
			wantHost:  "",
			wantPort:  0,
		},
		{
			name:      "valid JSON with all fields",
			extData:   `{"protocol":"ldap","host":"ldap.example.com","port":636,"client_id":"cn=admin"}`,
			wantProto: "ldap",
			wantHost:  "ldap.example.com",
			wantPort:  636,
			wantCID:   "cn=admin",
		},
		{
			name:      "JSON without protocol defaults to http",
			extData:   `{"host":"auth.example.com","port":8080}`,
			wantProto: "http",
			wantHost:  "auth.example.com",
			wantPort:  8080,
		},
		{
			name:    "invalid JSON returns error (P1: 不再吞错降级为 http 空配置)",
			extData: `{not valid json`,
			wantErr: true,
		},
		{
			name:      "JSON with ldaps protocol",
			extData:   `{"protocol":"ldaps","host":"secure.example.com"}`,
			wantProto: "ldaps",
			wantHost:  "secure.example.com",
		},
		{
			name:      "JSON with https and token_url",
			extData:   `{"protocol":"https","host":"sso.example.com","token_url":"/api/login","http_method":"POST"}`,
			wantProto: "https",
			wantHost:  "sso.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseExtConfig(tt.extData)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for invalid extData, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Protocol != tt.wantProto {
				t.Errorf("Protocol = %q, want %q", cfg.Protocol, tt.wantProto)
			}
			if cfg.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", cfg.Host, tt.wantHost)
			}
			if cfg.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", cfg.Port, tt.wantPort)
			}
			if tt.wantCID != "" && cfg.ClientID != tt.wantCID {
				t.Errorf("ClientID = %q, want %q", cfg.ClientID, tt.wantCID)
			}
		})
	}
}

func TestSsoGetStr(t *testing.T) {
	tests := []struct {
		name       string
		m          map[string]interface{}
		key        string
		defaultVal string
		want       string
	}{
		{
			name:       "nil map returns default",
			m:          nil,
			key:        "username",
			defaultVal: "fallback",
			want:       "fallback",
		},
		{
			name:       "key exists with string value",
			m:          map[string]interface{}{"username": "alice"},
			key:        "username",
			defaultVal: "",
			want:       "alice",
		},
		{
			name:       "key missing returns default",
			m:          map[string]interface{}{"username": "alice"},
			key:        "email",
			defaultVal: "none",
			want:       "none",
		},
		{
			name:       "key exists but non-string value returns default",
			m:          map[string]interface{}{"age": 30},
			key:        "age",
			defaultVal: "unknown",
			want:       "unknown",
		},
		{
			name:       "empty string value is returned as-is",
			m:          map[string]interface{}{"email": ""},
			key:        "email",
			defaultVal: "default@test.com",
			want:       "",
		},
		{
			name:       "key exists with bool value returns default",
			m:          map[string]interface{}{"active": true},
			key:        "active",
			defaultVal: "no",
			want:       "no",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ssoGetStr(tt.m, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("ssoGetStr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantP  int
		wantOK bool
	}{
		{
			name:   "valid port number",
			input:  "8080",
			wantP:  8080,
			wantOK: true,
		},
		{
			name:   "standard HTTP port",
			input:  "80",
			wantP:  80,
			wantOK: true,
		},
		{
			name:   "standard HTTPS port",
			input:  "443",
			wantP:  443,
			wantOK: true,
		},
		{
			name:   "empty string",
			input:  "",
			wantP:  0,
			wantOK: false,
		},
		{
			name:   "non-numeric string",
			input:  "abc",
			wantP:  0,
			wantOK: false,
		},
		{
			name:   "mixed alphanumeric Sscanf parses leading digits",
			input:  "80abc",
			wantP:  80,
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, ok := parsePort(tt.input)
			if ok != tt.wantOK {
				t.Errorf("parsePort(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if p != tt.wantP {
				t.Errorf("parsePort(%q) = %d, want %d", tt.input, p, tt.wantP)
			}
		})
	}
}

func TestIsTokenBlacklisted(t *testing.T) {
	t.Run("unknown token is not blacklisted", func(t *testing.T) {
		if IsTokenBlacklisted("never-blacklisted-token-xyz") {
			t.Error("expected unknown token to NOT be blacklisted")
		}
	})

	t.Run("blacklisted token is detected via Logout", func(t *testing.T) {
		token := "logout-test-token-abc"
		// Should not be blacklisted before logout
		if IsTokenBlacklisted(token) {
			t.Error("expected token to NOT be blacklisted before logout")
		}
		// Logout blacklists the token
		svc := &AuthService{}
		_ = svc.Logout(token)
		// Should now be blacklisted
		if !IsTokenBlacklisted(token) {
			t.Error("expected token to be blacklisted after Logout")
		}
	})

	t.Run("other tokens are not affected", func(t *testing.T) {
		svc := &AuthService{}
		_ = svc.Logout("only-this-token-blacklisted")
		if IsTokenBlacklisted("different-token-not-blacklisted") {
			t.Error("expected different token to NOT be blacklisted")
		}
	})
}
