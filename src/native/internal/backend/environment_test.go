package backend

import "testing"

// TestNewEnvironmentAcceptsAndRejectsOrigins checks each rule of the desktop app's origin policy: HTTPS outside
// development, loopback HTTP for development, no credentials, no path, query or fragment, default ports, and
// the production host.
func TestNewEnvironmentAcceptsAndRejectsOrigins(t *testing.T) {
	tests := []struct {
		name     string
		kind     EnvironmentKind
		raw      string
		packaged bool
		want     string
		wantErr  bool
	}{
		{name: "production default", kind: EnvironmentProduction, want: "https://api.nvm.fyi"},
		{name: "production trailing slash is dropped", kind: EnvironmentProduction, raw: "https://api.nvm.fyi/", want: "https://api.nvm.fyi"},
		{name: "production other host", kind: EnvironmentProduction, raw: "https://example.com", wantErr: true},
		{name: "production over http", kind: EnvironmentProduction, raw: "http://api.nvm.fyi", wantErr: true},
		{name: "development default", kind: EnvironmentDevelopment, want: "http://localhost:4321"},
		{name: "development loopback IPv4", kind: EnvironmentDevelopment, raw: "http://127.0.0.1:8787", want: "http://127.0.0.1:8787"},
		{name: "development remote host", kind: EnvironmentDevelopment, raw: "http://backend.example.com", wantErr: true},
		{name: "development over https loopback", kind: EnvironmentDevelopment, raw: "https://localhost:4321", wantErr: true},
		{name: "development refused when packaged", kind: EnvironmentDevelopment, packaged: true, wantErr: true},
		{name: "preview over https", kind: EnvironmentPRPreview, raw: "https://nvm-pr-42.example.app", want: "https://nvm-pr-42.example.app"},
		{name: "preview requires a URL", kind: EnvironmentPRPreview, raw: "  ", wantErr: true},
		{name: "custom over http refused", kind: EnvironmentCustom, raw: "http://staging.example.com", wantErr: true},
		{name: "custom with explicit default port", kind: EnvironmentCustom, raw: "https://staging.example.com:443", wantErr: true},
		{name: "custom with non-default port", kind: EnvironmentCustom, raw: "https://staging.example.com:8443", wantErr: true},
		{name: "credentials refused", kind: EnvironmentCustom, raw: "https://user:pass@staging.example.com", wantErr: true},
		{name: "path refused", kind: EnvironmentCustom, raw: "https://staging.example.com/api", wantErr: true},
		{name: "query refused", kind: EnvironmentCustom, raw: "https://staging.example.com?x=1", wantErr: true},
		{name: "fragment refused", kind: EnvironmentCustom, raw: "https://staging.example.com#x", wantErr: true},
		{name: "unknown scheme refused", kind: EnvironmentCustom, raw: "ftp://staging.example.com", wantErr: true},
		{name: "unknown kind refused", kind: EnvironmentKind("staging"), raw: "https://staging.example.com", wantErr: true},
		{name: "packaged refuses localhost preview", kind: EnvironmentCustom, raw: "https://localhost", packaged: true, wantErr: true},
		{name: "packaged refuses private address", kind: EnvironmentCustom, raw: "https://10.0.0.5", packaged: true, wantErr: true},
		{name: "packaged accepts public host", kind: EnvironmentCustom, raw: "https://staging.example.com", packaged: true, want: "https://staging.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewEnvironment(tt.kind, tt.raw, tt.packaged)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewEnvironment(%q, %q) = %+v, want an error", tt.kind, tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewEnvironment(%q, %q): %v", tt.kind, tt.raw, err)
			}
			if got.BaseURL != tt.want {
				t.Fatalf("BaseURL = %q, want %q", got.BaseURL, tt.want)
			}
			if got.Kind != tt.kind {
				t.Fatalf("Kind = %q, want %q", got.Kind, tt.kind)
			}
		})
	}
}

// TestIsUnsafeHostClassifiesNamesAndLiterals covers the hosts a packaged build must refuse. DNS is not resolved,
// so only names and literals are judged.
func TestIsUnsafeHostClassifiesNamesAndLiterals(t *testing.T) {
	tests := map[string]bool{
		"localhost":            true,
		"api.localhost":        true,
		"127.0.0.1":            true,
		"::1":                  true,
		"10.1.2.3":             true,
		"172.20.0.9":           true,
		"192.168.1.10":         true,
		"169.254.10.1":         true,
		"100.64.0.1":           true,
		"198.18.0.7":           true,
		"0.0.0.0":              true,
		"224.0.0.1":            true,
		"fe80::1":              true,
		"fd00::1":              true,
		"::ffff:10.0.0.1":      true,
		"api.nvm.fyi":          false,
		"staging.example.com":  false,
		"93.184.216.34":        false,
		"2606:4700:4700::1111": false,
	}
	for host, want := range tests {
		if got := isUnsafeHost(host); got != want {
			t.Errorf("isUnsafeHost(%q) = %v, want %v", host, got, want)
		}
	}
}
