package aichat

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
)

// TestEnterOnTheSignedOutGateStartsTheSignInAndSendsNothing checks that Enter starts the sign-in whatever was typed,
// and that the message is not sent.
func TestEnterOnTheSignedOutGateStartsTheSignInAndSendsNothing(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("", chat.TierFast)
	r.signed = false
	view := ChatView(r.deps(), record.ID)
	r.open(view)

	view.OnSubmit("hello")

	if r.signInStarts != 1 {
		t.Fatalf("sign-ins started = %d, want one", r.signInStarts)
	}
	if after, _ := r.service.Chat(record.ID); len(after.Messages) != 0 {
		t.Fatalf("messages = %d, want none while signed out", len(after.Messages))
	}
}

// TestSignedOutGateNamesTheEnterAction checks the footer names the sign-in as the Enter action.
func TestSignedOutGateNamesTheEnterAction(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("", chat.TierFast)
	r.signed = false
	view := ChatView(r.deps(), record.ID)

	if got := view.HintFn(); got != signInHint {
		t.Fatalf("footer = %q, want %q", got, signInHint)
	}
}

// TestSignInRefusalsSayWhy checks a sign-in that does not start says the reason in words, and a busy flow says to
// approve it in the browser.
func TestSignInRefusalsSayWhy(t *testing.T) {
	if got := signInRefusal(auth.ErrSignInInProgress); got != toastSignInBusy {
		t.Fatalf("busy refusal = %q, want %q", got, toastSignInBusy)
	}
	if got := signInRefusal(errors.New("the account is not available")); got != "the account is not available" {
		t.Fatalf("other refusal = %q", got)
	}
}

// TestSignInStatusFollowsTheFlow checks the gate's status line for each phase of the flow.
func TestSignInStatusFollowsTheFlow(t *testing.T) {
	cases := map[auth.Phase]string{
		auth.PhaseIdle:      "",
		auth.PhaseStarting:  "Requesting a one-time sign-in code…",
		auth.PhasePending:   "Approve this device in your browser. This view updates automatically.",
		auth.PhaseDenied:    "Sign-in was denied. Press Enter to try again.",
		auth.PhaseExpired:   "This one-time code expired. Press Enter to start a new sign-in.",
		auth.PhaseCancelled: "The sign-in was cancelled. Press Enter to try again.",
	}
	for phase, want := range cases {
		if got := signInStatus(auth.SignInState{Phase: phase}); got != want {
			t.Fatalf("status(%s) = %q, want %q", phase, got, want)
		}
	}
	if got := signInStatus(auth.SignInState{Phase: auth.PhaseFailed, Message: "Check your connection."}); got != "Check your connection." {
		t.Fatalf("failed status = %q, want the flow's message", got)
	}
}
