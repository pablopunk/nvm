package app

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/palette/feedback"
)

// TestFeedbackWithoutAnActionTitleSaysNevermind pins the HUD's title for a message no action named: the app's name.
func TestFeedbackWithoutAnActionTitleSaysNevermind(t *testing.T) {
	if got := feedbackTitle(""); got != "Nevermind" {
		t.Fatalf("untitled feedback reads %q, want Nevermind", got)
	}
	if got := feedbackTitle("Paste"); got != "Paste" {
		t.Fatalf("paste feedback reads %q, want Paste", got)
	}
}

// TestAFailedPasteIsAnErrorUnderTheTitleOfItsAction pins the paste feedback to 1.x: the action's title, an error tone.
func TestAFailedPasteIsAnErrorUnderTheTitleOfItsAction(t *testing.T) {
	f := pasteFailedFeedback(pasteActionTitle, errors.New("paste refused"))
	if f.Title != "Paste" || f.Tone != feedback.ToneError || f.Message != pasteFailureMessage(errors.New("paste refused")) {
		t.Fatalf("paste failure = %+v, want the Paste title with an error tone", f)
	}
}

// TestSignOutReadsAsErrorOnlyWhenTheServerDidNotRevoke pins the account feedback to 1.x's tones.
func TestSignOutReadsAsErrorOnlyWhenTheServerDidNotRevoke(t *testing.T) {
	if signOutTone(true) != feedback.ToneSuccess || signOutTone(false) != feedback.ToneError {
		t.Fatalf("sign-out tones are revoked %v and not revoked %v, want success and error", signOutTone(true), signOutTone(false))
	}
}
