package dictation

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestFailureMessagesSayWhatToDo(t *testing.T) {
	spendCap := &backend.Error{Kind: backend.KindServer, Status: 503, Code: "free_spend_cap_reached", Message: "paused"}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"no microphone", platform.ErrNotFound, "No microphone was found"},
		{"microphone denied", platform.ErrDenied, "Microphone access is denied. Enable it in System Settings"},
		{"microphone unsupported", platform.ErrUnsupported, "not available on this system yet"},
		{"codec missing", fmt.Errorf("%w: %w", ErrCodec, platform.ErrUnsupported), "libopus"},
		{"signed out", &backend.Error{Kind: backend.KindUnauthorized, Status: 401}, "Sign in to use API dictation."},
		{"no credits", &backend.Error{Kind: backend.KindInsufficientCredits, Status: 402, Message: "Not enough credits for API dictation."}, "Not enough credits"},
		{"rate limited", &backend.Error{Kind: backend.KindRateLimited, Status: 429}, "Too many dictations"},
		{"timed out", &backend.Error{Kind: backend.KindTimeout}, "timed out"},
		{"offline", &backend.Error{Kind: backend.KindOffline}, "Check your connection"},
		{"too old", &backend.Error{Kind: backend.KindIncompatible, Status: 426}, "too old"},
		{"backend down", &backend.Error{Kind: backend.KindServer, Status: 503}, "unavailable"},
		{"free pool paused", spendCap, "temporarily paused"},
		{"per-request limit", &backend.Error{Kind: backend.KindRejected, Status: 400, Message: "This recording exceeds the 30-credit per-request limit."}, "per-request limit"},
		{"unknown", errors.New("boom"), "Dictation failed"},
		{"busy", ErrBusy, "already running"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FailureMessage(tt.err)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("FailureMessage = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}

func TestACancelledDictationHasNoMessage(t *testing.T) {
	if got := FailureMessage(ErrCancelled); got != "" {
		t.Fatalf("FailureMessage(cancelled) = %q, want nothing to show", got)
	}
}
