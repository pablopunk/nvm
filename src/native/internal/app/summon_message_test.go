package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestAPaletteKeyHeldByAnActionIsNamedAsThatAction(t *testing.T) {
	err := errors.New("Control+Shift+Y is already used by Clipboard History")
	got := paletteChangeFailureMessage(err, "Control+Shift+Y", "Alt+Shift+Space")
	if got != "Control+Shift+Y is already used by Clipboard History" {
		t.Fatalf("message = %q, want the action that holds the key", got)
	}
}

func TestAPaletteKeyTheOSRefusesIsTakenByAnotherApp(t *testing.T) {
	err := fmt.Errorf("%w: Control+Shift+Space", platform.ErrConflict)
	got := paletteChangeFailureMessage(err, "Control+Shift+Space", "Alt+Shift+Space")
	want := "“Control+Shift+Space” is taken by another app or is not a shortcut. Alt+Shift+Space stays."
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}
