package logx

import (
	"errors"
	"testing"
)

func TestIsSensitiveKeyMatchesSpellingsOfSecretNames(t *testing.T) {
	r := NewRedactor()
	for _, key := range []string{"token", "accessToken", "api_key", "API-Key", "apikey", "clientSecret", "password", "Authorization", "session_cookie", "passwd"} {
		if !r.IsSensitiveKey(key) {
			t.Errorf("IsSensitiveKey(%q) = false, want true", key)
		}
	}
	for _, key := range []string{"count", "name", "url", "duration_ms", "provider"} {
		if r.IsSensitiveKey(key) {
			t.Errorf("IsSensitiveKey(%q) = true, want false", key)
		}
	}
}

func TestTextRemovesSecretsAndKeepsProse(t *testing.T) {
	longBase64 := "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789+/ab"
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"authorization header", "Authorization: Basic dXNlcjpwYXNz", "Authorization: [REDACTED]"},
		{"bearer token", "sent Bearer abc.def-123", "sent Bearer [REDACTED]"},
		{"key value pair", "password=hunter2 user=pol", "password=[REDACTED] user=pol"},
		{"quoted json value", `{"token":"xyz","ok":true}`, `{"token":[REDACTED],"ok":true}`},
		{"url query values", "GET https://h.test/cb?code=AAA&state=ok", "GET https://h.test/cb?code=[REDACTED]&state=[REDACTED]"},
		{"url fragment values", "https://h.test/#access_token=ZZZ", "https://h.test/#access_token=[REDACTED]"},
		{"provider prefix", "key sk-live_ABCDEFGHIJKL1234 used", "key [REDACTED] used"},
		{"long hex run", "id 0123456789abcdef0123456789abcdef", "id [REDACTED]"},
		{"long base64 run", "k=" + longBase64, "k=[REDACTED]"},
		{"plain error text", "could not open settings.json: permission denied", "could not open settings.json: permission denied"},
		{"long lowercase path", "/home/pol/src/nvm/native/internal/platform/fake/clock_test.go", "/home/pol/src/nvm/native/internal/platform/fake/clock_test.go"},
		{"all-digit run", "took 12345678901234567890123456789012 ns", "took 12345678901234567890123456789012 ns"},
		{"already redacted stays stable", "token=[REDACTED]", "token=[REDACTED]"},
	}
	r := NewRedactor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.Text(tt.in); got != tt.want {
				t.Fatalf("Text(%q)\n got: %q\nwant: %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValueRedactsByKeyAndNormalizesStructs(t *testing.T) {
	r := NewRedactor()
	if got := r.Value("apiKey", "anything"); got != RedactedValue {
		t.Fatalf("sensitive key value = %v, want %s", got, RedactedValue)
	}
	if got := r.Value("duration_ms", 12); got != 12 {
		t.Fatalf("plain number = %v, want 12", got)
	}
	if got := r.Value("err", errors.New("token=abc")); got != "token=[REDACTED]" {
		t.Fatalf("error text = %v", got)
	}
	type payload struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	}
	got, ok := r.Value("payload", payload{Token: "t0k3n", Name: "pol"}).(map[string]any)
	if !ok {
		t.Fatal("struct did not normalize to an object")
	}
	if got["token"] != RedactedValue || got["name"] != "pol" {
		t.Fatalf("struct = %v, want token removed and name kept", got)
	}
}

func TestValueKeepsNonFiniteFloatsEncodable(t *testing.T) {
	r := NewRedactor()
	if got := r.Value("ratio", nan()); got != "NaN" {
		t.Fatalf("NaN = %v, want the text NaN", got)
	}
}

func nan() float64 {
	zero := 0.0
	return zero / zero
}
