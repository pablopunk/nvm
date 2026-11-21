package backend

import (
	"context"
	"net/http"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestRevokeCurrentTokenSendsADeleteWithTheBearer checks the route, the method and the bearer the revocation
// carries, and that a 204 counts as revoked.
func TestRevokeCurrentTokenSendsADeleteWithTheBearer(t *testing.T) {
	h := newHarness(t)
	h.http.Script(fake.Response{Status: http.StatusNoContent})
	if err := h.client.RevokeCurrentToken(context.Background()); err != nil {
		t.Fatalf("RevokeCurrentToken: %v", err)
	}
	request := h.http.Requests()[0]
	if request.Method != http.MethodDelete || request.URL != "https://api.nvm.fyi/api/tokens/current" {
		t.Fatalf("request = %s %s, want DELETE /api/tokens/current", request.Method, request.URL)
	}
	if got := request.Header.Get("Authorization"); got != "Bearer "+canaryToken {
		t.Fatalf("Authorization = %q, want the bearer token", got)
	}
}

// TestRevokeCurrentTokenTreatsUnauthorizedAsRevoked checks that a token the backend already rejects is not an
// error, and that other failures are returned.
func TestRevokeCurrentTokenTreatsUnauthorizedAsRevoked(t *testing.T) {
	h := newHarness(t)
	h.http.Script(fake.Response{Status: http.StatusUnauthorized, Body: []byte("Unauthorized")})
	if err := h.client.RevokeCurrentToken(context.Background()); err != nil {
		t.Fatalf("401 should count as revoked, got %v", err)
	}
	h.http.Script(fake.Response{Status: http.StatusBadGateway, Body: []byte("bad gateway")})
	if err := h.client.RevokeCurrentToken(context.Background()); !IsKind(err, KindServer) {
		t.Fatalf("502 error = %v, want KindServer", err)
	}
}
