package backend

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestFixtureFailureModesMapToKinds drives each HTTP failure mode of the fixture backend through an idempotent
// GET and checks the kind, the status, the details the error carries, and how many attempts the client made.
// Server errors are transient and retried; the others must each cost exactly one request.
func TestFixtureFailureModesMapToKinds(t *testing.T) {
	tests := []struct {
		name         string
		mode         fixtures.FailureMode
		wantKind     Kind
		wantStatus   int
		wantRequests int
		check        func(t *testing.T, err *Error)
	}{
		{
			name:         "401 unauthorized is not retried",
			mode:         fixtures.ModeUnauthorized,
			wantKind:     KindUnauthorized,
			wantStatus:   401,
			wantRequests: 1,
		},
		{
			name:         "402 carries the estimate and balance",
			mode:         fixtures.ModeInsufficientCredits,
			wantKind:     KindInsufficientCredits,
			wantStatus:   402,
			wantRequests: 1,
			check: func(t *testing.T, err *Error) {
				if err.CreditsInfo == nil || err.CreditsInfo.EstimatedCredits != 95 || err.CreditsInfo.Balance != 81 {
					t.Fatalf("CreditsInfo = %+v, want estimate 95 and balance 81", err.CreditsInfo)
				}
			},
		},
		{
			name:         "429 carries the retry-after wait and is not retried",
			mode:         fixtures.ModeRateLimited,
			wantKind:     KindRateLimited,
			wantStatus:   429,
			wantRequests: 1,
			check: func(t *testing.T, err *Error) {
				if err.RetryAfter != 30*time.Second || !err.Retryable {
					t.Fatalf("RetryAfter = %v retryable = %v, want 30s and retryable", err.RetryAfter, err.Retryable)
				}
			},
		},
		{
			name:         "500 is retried until the attempt budget ends",
			mode:         fixtures.ModeServerError,
			wantKind:     KindServer,
			wantStatus:   500,
			wantRequests: 3,
			check: func(t *testing.T, err *Error) {
				if !err.Retryable {
					t.Fatal("a 500 should be marked retryable")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			backend := h.withBackend(t)
			backend.SetMode(tt.mode)
			h.useSynchronousWaits()
			_, err := h.client.ActiveModel(context.Background(), ActiveModelQuery{})
			typed := asError(err)
			if typed == nil {
				t.Fatalf("ActiveModel error = %v, want a *Error", err)
			}
			if typed.Kind != tt.wantKind || typed.Status != tt.wantStatus {
				t.Fatalf("got kind %s status %d, want %s status %d (%v)", typed.Kind, typed.Status, tt.wantKind, tt.wantStatus, typed)
			}
			if n := len(backend.Requests()); n != tt.wantRequests {
				t.Fatalf("made %d requests, want %d", n, tt.wantRequests)
			}
			if message := UserMessage(err); message == "" || message == copyUnknown {
				t.Fatalf("UserMessage = %q, want a specific line", message)
			}
			if tt.check != nil {
				tt.check(t, typed)
			}
		})
	}
}

// TestFixtureHealthyBackendAnswersWithoutError is the control for the table above: the same route with no
// failure mode answers once.
func TestFixtureHealthyBackendAnswersWithoutError(t *testing.T) {
	h := newHarness(t)
	backend := h.withBackend(t)
	if _, err := h.client.ActiveModel(context.Background(), ActiveModelQuery{}); err != nil {
		t.Fatalf("ActiveModel on a healthy backend: %v", err)
	}
	if n := len(backend.Requests()); n != 1 {
		t.Fatalf("made %d requests, want 1", n)
	}
}

// hangingHTTP accepts every request and never answers, as a stalled network does. Each request advances the fake
// clock past every deadline, so the client's own deadline fires inside the request; the request then ends only
// because the client cancelled it. Nothing here depends on goroutine scheduling order.
type hangingHTTP struct {
	clock    *fake.Clock
	requests int
}

// Do advances the clock, then waits for the request's context to end.
func (h *hangingHTTP) Do(req *platform.Request) (*platform.Response, error) {
	h.requests++
	h.clock.Advance(time.Minute)
	<-req.Context().Done()
	return nil, req.Context().Err()
}

// TestStalledBackendTimesOutAndRetriesOnlyGets checks a request that never answers is reported as a timeout. A GET
// is retried with backoff until the attempt budget is spent; a POST is attempted once.
func TestStalledBackendTimesOutAndRetriesOnlyGets(t *testing.T) {
	h := newHarness(t)
	hang := &hangingHTTP{clock: h.clock}
	h.client.http = hang
	waits := h.useSynchronousWaits()

	_, err := h.client.ActiveModel(context.Background(), ActiveModelQuery{})
	if !IsKind(err, KindTimeout) || UserMessage(err) != copyTimeout {
		t.Fatalf("ActiveModel error = %v, want KindTimeout with the timeout line", err)
	}
	if hang.requests != 3 {
		t.Fatalf("GET made %d attempts, want 3", hang.requests)
	}
	if want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond}; !slices.Equal(*waits, want) {
		t.Fatalf("waits = %v, want %v", *waits, want)
	}

	hang.requests = 0
	_, err = h.client.StartDeviceSignIn(context.Background())
	if !IsKind(err, KindTimeout) {
		t.Fatalf("StartDeviceSignIn error = %v, want KindTimeout", err)
	}
	if hang.requests != 1 {
		t.Fatalf("POST made %d attempts, want 1", hang.requests)
	}
}

// TestCompatibilityOnAnOlderBackendIsUnknownNotAnError checks a deployment without the manifest route does not
// block the app: the fixture answers 404 for it.
func TestCompatibilityOnAnOlderBackendIsUnknownNotAnError(t *testing.T) {
	h := newHarness(t)
	h.withBackend(t)
	report, err := h.client.Compatibility(context.Background())
	if err != nil {
		t.Fatalf("Compatibility on a 404 route: %v", err)
	}
	if report.State != CompatUnknown {
		t.Fatalf("State = %q, want %q", report.State, CompatUnknown)
	}
}
