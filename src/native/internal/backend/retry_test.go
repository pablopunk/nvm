package backend

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// errNetworkDown is a transport failure that is not a timeout, so it classifies as Offline.
var errNetworkDown = errors.New("dial tcp: connection refused")

// TestDefaultRetryDelaysDoubleAndCap pins the backoff schedule: the base doubles per retry and stops at the cap.
func TestDefaultRetryDelaysDoubleAndCap(t *testing.T) {
	policy := defaultRetryPolicy()
	want := map[int]time.Duration{
		1: 250 * time.Millisecond,
		2: 500 * time.Millisecond,
		3: time.Second,
		4: 2 * time.Second,
		5: 4 * time.Second,
		9: 4 * time.Second,
	}
	for retry, delay := range want {
		if got := policy.delay(retry); got != delay {
			t.Errorf("delay(%d) = %v, want %v", retry, got, delay)
		}
	}
	if got := policy.delay(0); got != 0 {
		t.Errorf("delay(0) = %v, want 0", got)
	}
}

// TestDefaultJitterStaysWithinHalfToWhole checks jitter never shortens a delay below half nor lengthens it.
func TestDefaultJitterStaysWithinHalfToWhole(t *testing.T) {
	const base = 2 * time.Second
	for range 500 {
		got := defaultJitter(base)
		if got < base/2 || got > base {
			t.Fatalf("defaultJitter(%v) = %v, want within [%v, %v]", base, got, base/2, base)
		}
	}
	if got := defaultJitter(0); got != 0 {
		t.Fatalf("defaultJitter(0) = %v, want 0", got)
	}
}

// TestIdempotentGetRetriesThroughTransientFailures checks a GET survives a network failure and a server error,
// waits the backoff between attempts, and succeeds on the third.
func TestIdempotentGetRetriesThroughTransientFailures(t *testing.T) {
	h := newHarness(t)
	waits := h.useSynchronousWaits()
	h.http.Script(
		fake.Response{Err: errNetworkDown},
		fake.Response{Status: http.StatusServiceUnavailable, Body: []byte(`{"error":{"type":"service_unavailable","message":"busy"}}`)},
		jsonBody(http.StatusOK, string(testManifest(nil))),
	)
	report, err := h.client.Compatibility(context.Background())
	if err != nil {
		t.Fatalf("Compatibility after retries: %v", err)
	}
	if report.State != CompatOK {
		t.Fatalf("State = %q, want %q", report.State, CompatOK)
	}
	if n := len(h.http.Requests()); n != 3 {
		t.Fatalf("made %d requests, want 3", n)
	}
	if want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond}; !slices.Equal(*waits, want) {
		t.Fatalf("waits = %v, want %v", *waits, want)
	}
}

// TestRetriesStopAfterTheAttemptBudget checks a GET that keeps failing gives up after the configured attempts,
// waiting between them, and returns the last failure.
func TestRetriesStopAfterTheAttemptBudget(t *testing.T) {
	h := newHarness(t)
	waits := h.useSynchronousWaits()
	h.http.Script(fake.Response{Err: errNetworkDown}, fake.Response{Err: errNetworkDown}, fake.Response{Err: errNetworkDown})
	_, err := h.client.Compatibility(context.Background())
	if !IsKind(err, KindOffline) {
		t.Fatalf("Compatibility error = %v, want KindOffline", err)
	}
	if n := len(h.http.Requests()); n != 3 {
		t.Fatalf("made %d requests, want 3 (the default attempt budget)", n)
	}
	if want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond}; !slices.Equal(*waits, want) {
		t.Fatalf("waits = %v, want %v", *waits, want)
	}
}

// TestPostIsNeverRetried checks a POST makes one attempt even when the failure is transient, so a stream or
// a billed call cannot run twice.
func TestPostIsNeverRetried(t *testing.T) {
	h := newHarness(t)
	h.http.Script(fake.Response{Err: errNetworkDown}, jsonBody(http.StatusOK, initiateJSON(epochPlus(fiveMinutes), "2000")))
	_, err := h.client.StartDeviceSignIn(context.Background())
	if !IsKind(err, KindOffline) {
		t.Fatalf("StartDeviceSignIn error = %v, want KindOffline", err)
	}
	if n := len(h.http.Requests()); n != 1 {
		t.Fatalf("made %d requests, want exactly 1 for a POST", n)
	}
}

// TestRateLimitIsNotRetried checks a 429 is reported with its wait rather than retried in a loop.
func TestRateLimitIsNotRetried(t *testing.T) {
	h := newHarness(t)
	h.http.Script(fake.Response{
		Status: http.StatusTooManyRequests,
		Header: http.Header{"Retry-After": []string{"12"}},
		Body:   []byte(`{"error":{"type":"rate_limited","message":"slow down"}}`),
	})
	_, err := h.client.Compatibility(context.Background())
	typed := asError(err)
	if typed == nil || typed.Kind != KindRateLimited || typed.RetryAfter != 12*time.Second {
		t.Fatalf("Compatibility error = %v, want KindRateLimited with a 12s wait", err)
	}
	if n := len(h.http.Requests()); n != 1 {
		t.Fatalf("made %d requests, want 1", n)
	}
}

// TestCancelledCallDoesNotRetry checks a caller who cancels is not retried, however transient the failure.
func TestCancelledCallDoesNotRetry(t *testing.T) {
	h := newHarness(t)
	h.http.Script(fake.Response{Err: errNetworkDown}, jsonBody(http.StatusOK, string(testManifest(nil))))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.client.Compatibility(ctx)
	if !IsKind(err, KindCanceled) {
		t.Fatalf("Compatibility with a cancelled context = %v, want KindCanceled", err)
	}
	if n := len(h.http.Requests()); n > 1 {
		t.Fatalf("made %d requests after cancellation, want at most 1", n)
	}
}
