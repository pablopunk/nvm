package backend

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// allKinds lists every failure kind, so copy tests cover the whole taxonomy.
var allKinds = []Kind{
	KindOffline, KindTimeout, KindCanceled, KindUnauthorized, KindInsufficientCredits,
	KindRateLimited, KindServer, KindIncompatible, KindRejected, KindBadResponse,
}

// TestEveryKindHasItsOwnPaletteLine checks each failure kind maps to a non-empty line and no two kinds share one,
// so a user never sees the same line for two different problems.
func TestEveryKindHasItsOwnPaletteLine(t *testing.T) {
	seen := map[string]Kind{}
	for _, kind := range allKinds {
		message := UserMessage(&Error{Kind: kind, Status: 500})
		if message == "" {
			t.Errorf("kind %s has no palette line", kind)
		}
		if previous, dup := seen[message]; dup {
			t.Errorf("kinds %s and %s share the line %q", previous, kind, message)
		}
		seen[message] = kind
	}
}

// TestCopyStaysInActiveVoiceAndSaysWhatToDo checks every line ends with a full stop and is not a raw error
// placeholder. The full list is in copy.go for review.
func TestCopyStaysInActiveVoiceAndSaysWhatToDo(t *testing.T) {
	lines := []string{
		copyOffline, copyTimeout, copyCanceled, copyUnauthorized, copyCreditsDetailed, copyCreditsGeneric,
		copyRateLimitedDetailed, copyRateLimitedGeneric, copyServer, copyFreeSpendPaused, copyModelUnavailable,
		copyIncompatible, copyIncompatibleAt, copyRequestShape, copyRequestTooLarge, copyDecisionRequest,
		copyDecisionAnswer, copyRejected, copyBadResponse, copySignInExpired, copySignInDenied, copyUnknown,
	}
	for _, line := range lines {
		if !strings.HasSuffix(line, ".") {
			t.Errorf("copy %q does not end with a full stop", line)
		}
		if strings.Contains(line, "%!") || strings.Contains(line, "error:") {
			t.Errorf("copy %q looks like an unformatted or raw error", line)
		}
	}
}

// TestCreditsLineNamesTheCostAndBalance checks an insufficient-credits line carries the numbers the backend sent.
func TestCreditsLineNamesTheCostAndBalance(t *testing.T) {
	err := &Error{Kind: KindInsufficientCredits, Status: 402, CreditsInfo: &CreditsInfo{EstimatedCredits: 95, Balance: 81}}
	message := UserMessage(err)
	if !strings.Contains(message, "95") || !strings.Contains(message, "81") {
		t.Fatalf("UserMessage = %q, want the cost 95 and balance 81", message)
	}
	generic := UserMessage(&Error{Kind: KindInsufficientCredits, Status: 402})
	if generic != copyCreditsGeneric {
		t.Fatalf("UserMessage without numbers = %q, want the generic line", generic)
	}
}

// TestRateLimitLineRoundsTheWaitUp checks the wait is given in whole seconds, rounded up so the user never retries
// early.
func TestRateLimitLineRoundsTheWaitUp(t *testing.T) {
	message := UserMessage(&Error{Kind: KindRateLimited, Status: 429, RetryAfter: 1500 * time.Millisecond})
	if want := fmt.Sprintf(copyRateLimitedDetailed, 2); message != want {
		t.Fatalf("UserMessage = %q, want %q", message, want)
	}
}

// TestFreeSpendPauseHasItsOwnLine checks the paused free pool is named by its server code, not by the generic
// server line, because the user can act on it differently.
func TestFreeSpendPauseHasItsOwnLine(t *testing.T) {
	message := UserMessage(&Error{Kind: KindServer, Status: 503, Code: codeFreeSpendCapReached})
	if message != copyFreeSpendPaused {
		t.Fatalf("UserMessage = %q, want the paused-free-AI line", message)
	}
}

// TestSignInSentinelsHaveLines checks the device flow's terminal results read as instructions.
func TestSignInSentinelsHaveLines(t *testing.T) {
	if got := UserMessage(ErrSignInExpired); got != copySignInExpired {
		t.Fatalf("expired = %q", got)
	}
	if got := UserMessage(fmt.Errorf("wrapped: %w", ErrSignInDenied)); got != copySignInDenied {
		t.Fatalf("denied = %q", got)
	}
}

// TestRawErrorsNeverReachThePalette checks an error without a backend classification shows the generic line, and
// that its text is not shown.
func TestRawErrorsNeverReachThePalette(t *testing.T) {
	raw := errors.New("dial tcp 10.0.0.9:443: connect: refused")
	message := UserMessage(raw)
	if message != copyUnknown || strings.Contains(message, "10.0.0.9") {
		t.Fatalf("UserMessage(raw) = %q, want the generic line", message)
	}
	if UserMessage(nil) != "" {
		t.Fatal("UserMessage(nil) should be empty")
	}
}
