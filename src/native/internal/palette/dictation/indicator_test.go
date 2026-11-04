package dictation

import (
	"strings"
	"testing"
)

func TestHeightGrowsWithAFailureMessageAndNeverShrinksBelowTheBase(t *testing.T) {
	short := Height(State{Mode: ModeFailed, Detail: "Sign in."})
	long := Height(State{Mode: ModeFailed, Detail: strings.Repeat("word ", 40)})
	if short != BaseHeight {
		t.Fatalf("short failure height = %d, want the base %d", short, BaseHeight)
	}
	if long <= BaseHeight {
		t.Fatalf("long failure height = %d, want more than the base", long)
	}
	if got := Height(State{Mode: ModeListening, Level: 1}); got != BaseHeight {
		t.Fatalf("listening height = %d, want the base", got)
	}
}

func TestSubtitleKeepsTheWholeFailureMessage(t *testing.T) {
	message := "This recording exceeds the 30-credit per-request limit. Split it into shorter clips and retry."
	if got := Subtitle(State{Mode: ModeFailed, Detail: message}); got != message {
		t.Fatalf("Subtitle = %q, want the full message", got)
	}
}

func TestAButtonAddsARowToTheHeight(t *testing.T) {
	if got := Height(State{Mode: ModeListening, Control: ControlCancel}); got != BaseHeight+controlRowHeight {
		t.Fatalf("listening with cancel height = %d, want the base plus the button row", got)
	}
	if got := Height(State{Mode: ModeFailed, Detail: "Sign in.", Control: ControlMicrophoneSettings}); got != BaseHeight+controlRowHeight {
		t.Fatalf("failure with a button height = %d, want the base plus the button row", got)
	}
}
