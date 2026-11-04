package real

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// TestRefuseNonPublicDialChecksTheResolvedAddress: the connect-time hook refuses private destinations and allows
// public ones, judged by the address the socket is about to use.
func TestRefuseNonPublicDialChecksTheResolvedAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:80", "10.0.0.5:443", "169.254.169.254:80", "[::1]:443", "[fe80::1]:80"} {
		if err := refuseNonPublicDial("tcp", address, nil); !errors.Is(err, platform.ErrNonPublicAddress) {
			t.Errorf("%s: %v, want ErrNonPublicAddress", address, err)
		}
	}
	if err := refuseNonPublicDial("tcp", "93.184.216.34:443", nil); err != nil {
		t.Errorf("public address refused: %v", err)
	}
	if err := refuseNonPublicDial("tcp", "not-an-address", nil); err == nil {
		t.Error("an unparsed address was allowed")
	}
}

// TestPublicClientRefusesALoopbackServer runs a real listener on the loopback address and asks the public client
// for it. The connection is refused before any request is sent, so the server never sees one.
func TestPublicClientRefusesALoopbackServer(t *testing.T) {
	served := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served <- struct{}{} }))
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := newPublicHTTP().Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("the public client reached a loopback server")
	}
	if !errors.Is(err, platform.ErrNonPublicAddress) {
		t.Fatalf("Do = %v, want ErrNonPublicAddress", err)
	}
	select {
	case <-served:
		t.Fatal("the loopback server received a request")
	default:
	}
}

// TestPublicClientDoesNotFollowRedirects: the caller checks each hop, so the client returns the redirect itself.
func TestPublicClientDoesNotFollowRedirects(t *testing.T) {
	client := newPublicHTTP().(httpClient)
	if client.client.CheckRedirect == nil {
		t.Fatal("the public client follows redirects on its own")
	}
	if err := client.client.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("CheckRedirect = %v, want ErrUseLastResponse", err)
	}
}

// TestResolverAnswersLocalhostWithNonPublicAddresses: the production resolver answers for a name it can resolve
// locally, and every answer is one the public check refuses.
func TestResolverAnswersLocalhostWithNonPublicAddresses(t *testing.T) {
	addrs, err := resolver{}.LookupAddrs(context.Background(), "localhost")
	if err != nil {
		t.Skipf("localhost does not resolve here: %v", err)
	}
	for _, addr := range addrs {
		if platform.IsPublicAddress(addr) {
			t.Fatalf("localhost resolved to a public address %s", addr)
		}
	}
}
