package backend

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestConsecutiveNetworkFailuresFlipOfflineAndAnyResponseClears checks the offline flag turns on at the third
// network failure in a row and turns off on the next response, even an error response from a live server.
func TestConsecutiveNetworkFailuresFlipOfflineAndAnyResponseClears(t *testing.T) {
	h := newHarness(t)
	h.http.Script(
		fake.Response{Err: errNetworkDown},
		fake.Response{Err: errNetworkDown},
		fake.Response{Err: errNetworkDown},
		fake.Response{Status: 500, Body: []byte(`{"error":{"type":"internal_error","message":"boom"}}`)},
	)
	ctx := context.Background()
	for failure := 1; failure <= 2; failure++ {
		if _, err := h.client.StartDeviceSignIn(ctx); !IsKind(err, KindOffline) {
			t.Fatalf("failure %d: error = %v, want KindOffline", failure, err)
		}
		if !h.client.Online() {
			t.Fatalf("client went offline after %d failures, want the third", failure)
		}
	}
	if _, err := h.client.StartDeviceSignIn(ctx); !IsKind(err, KindOffline) {
		t.Fatalf("third failure: error = %v, want KindOffline", err)
	}
	if h.client.Online() {
		t.Fatal("client is still online after three consecutive network failures")
	}
	if _, err := h.client.StartDeviceSignIn(ctx); IsKind(err, KindOffline) {
		t.Fatalf("a live server answered 500, got %v", err)
	}
	if !h.client.Online() {
		t.Fatal("client stayed offline after the server answered")
	}
}

// TestCancelledCallsDoNotCountAsOffline checks a caller who cancels says nothing about the network, so the
// offline flag stays off.
func TestCancelledCallsDoNotCountAsOffline(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range 5 {
		if _, err := h.client.StartDeviceSignIn(ctx); !IsKind(err, KindCanceled) {
			t.Fatalf("error = %v, want KindCanceled", err)
		}
	}
	if !h.client.Online() {
		t.Fatal("cancelled calls turned the client offline")
	}
}
