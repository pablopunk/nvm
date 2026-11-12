package dictation

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// flakyTranscriber answers each call with the next scripted outcome: an error, or the text when the error is nil.
type flakyTranscriber struct {
	scriptedTranscriber
	mu       sync.Mutex
	outcomes []error
	calls    []Segment
}

func (f *flakyTranscriber) Transcribe(_ context.Context, segment Segment) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, segment)
	outcome := f.outcomes[min(len(f.calls), len(f.outcomes))-1]
	if outcome != nil {
		return "", outcome
	}
	return "recovered text", nil
}

func backendFailure(kind backend.Kind) error {
	return &backend.Error{Kind: kind, Message: string(kind)}
}

// retryHarness runs a dictation over a flaky transcriber with a recorder, and records the waits between attempts.
func retryHarness(t *testing.T, outcomes ...error) (harness, *flakyTranscriber, *fakeRecorder, *[]time.Duration) {
	t.Helper()
	h := newHarness(t)
	flaky := &flakyTranscriber{outcomes: outcomes}
	recorder := newFakeRecorder()
	var waits []time.Duration
	h.service.cfg.Transcriber = flaky
	h.service.cfg.Recorder = recorder
	h.service.cfg.Retry = RetryPolicy{Sleep: func(_ context.Context, wait time.Duration) error {
		waits = append(waits, wait)
		return nil
	}}
	return h, flaky, recorder, &waits
}

func dictate(h harness) (string, error) {
	h.mic.Queue(speech(0.5, 9000)...)
	_ = h.service.Start("default")
	return h.service.Stop()
}

func TestATransientFailureIsRetriedWithBackoffAndTheAudioGoesWhenItSucceeds(t *testing.T) {
	h, flaky, recorder, waits := retryHarness(t, backendFailure(backend.KindServer), backendFailure(backend.KindOffline), nil)
	text, err := dictate(h)
	if err != nil || text != "recovered text" {
		t.Fatalf("Stop = %q, %v; want the text after two transient failures", text, err)
	}
	if want := []time.Duration{time.Second, 2 * time.Second}; !slices.Equal(*waits, want) {
		t.Fatalf("waits = %v, want %v", *waits, want)
	}
	if flaky.calls[1].OperationID != flaky.calls[0].OperationID+"-r1" {
		t.Fatalf("a server failure retried with %q, want a fresh operation id", flaky.calls[1].OperationID)
	}
	if flaky.calls[2].OperationID != flaky.calls[1].OperationID {
		t.Fatalf("a lost connection retried with %q, want the same operation id", flaky.calls[2].OperationID)
	}
	if ids := recorder.savedIDs(); len(ids) != 0 || len(recorder.failed) != 0 {
		t.Fatalf("saved %v, failed %v after success; want the audio gone", ids, recorder.failed)
	}
}

func TestATransientFailureThatKeepsFailingStopsAfterTheAttemptsAndKeepsTheAudio(t *testing.T) {
	h, flaky, recorder, waits := retryHarness(t, backendFailure(backend.KindTimeout))
	_, err := dictate(h)
	var saved *SavedError
	if !errors.As(err, &saved) {
		t.Fatalf("Stop error = %v, want a SavedError", err)
	}
	if len(flaky.calls) != 3 || len(*waits) != 2 {
		t.Fatalf("%d attempts and %d waits, want 3 and 2", len(flaky.calls), len(*waits))
	}
	ids := recorder.savedIDs()
	if len(ids) != 1 || !strings.Contains(recorder.failed[ids[0]], "timed out") {
		t.Fatalf("saved %v, failed %v; want the audio kept with the timeout reason", ids, recorder.failed)
	}
}

func TestAPermanentFailureIsNotRetriedAndKeepsTheAudio(t *testing.T) {
	for _, kind := range []backend.Kind{backend.KindUnauthorized, backend.KindInsufficientCredits, backend.KindRateLimited, backend.KindRejected} {
		h, flaky, recorder, waits := retryHarness(t, backendFailure(kind))
		_, err := dictate(h)
		if err == nil {
			t.Fatalf("%s: Stop error = nil", kind)
		}
		if len(flaky.calls) != 1 || len(*waits) != 0 {
			t.Fatalf("%s: %d attempts and %d waits, want one attempt", kind, len(flaky.calls), len(*waits))
		}
		ids := recorder.savedIDs()
		if len(ids) != 1 || recorder.failed[ids[0]] == "" {
			t.Fatalf("%s: saved %v, failed %v; want the audio kept with a reason", kind, ids, recorder.failed)
		}
	}
}

func TestACancelledRetryWaitStopsRetrying(t *testing.T) {
	h, flaky, _, _ := retryHarness(t, backendFailure(backend.KindServer))
	h.service.cfg.Retry.Sleep = func(context.Context, time.Duration) error { return context.Canceled }
	if _, err := dictate(h); err == nil {
		t.Fatal("Stop succeeded, want the first failure")
	}
	if len(flaky.calls) != 1 {
		t.Fatalf("%d attempts after a cancelled wait, want 1", len(flaky.calls))
	}
}

func TestAManualRetryRecordsItsFailureAndKeepsTheAudio(t *testing.T) {
	h, _, recorder, _ := retryHarness(t, backendFailure(backend.KindUnauthorized))
	recorder.saved["dictation-saved"] = [][]byte{[]byte("one")}
	if _, err := h.service.Retry(context.Background(), "dictation-saved"); err == nil {
		t.Fatal("Retry succeeded, want the failure")
	}
	if recorder.failed["dictation-saved"] == "" || len(recorder.saved) != 1 {
		t.Fatalf("failed %v, saved %v; want the reason recorded and the audio kept", recorder.failed, recorder.saved)
	}
}
