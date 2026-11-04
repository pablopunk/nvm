package app

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/update"
)

// The palette hides after a command unless the command asks it to stay open. A check's answer is a toast, and a
// toast raised while the palette is hidden is dropped, so Check for Updates must keep the palette open.
func TestCheckForUpdatesKeepsThePaletteOpenToShowItsAnswer(t *testing.T) {
	p := goldenPalette(t)
	for _, cmd := range p.commands(false) {
		if cmd.Title == "Check for Updates" {
			if cmd.Dismiss != search.DismissNever {
				t.Fatalf("Check for Updates dismisses = %v, want DismissNever", cmd.Dismiss)
			}
			return
		}
	}
	t.Fatal("no Check for Updates command")
}

func TestCheckForUpdatesReportsTheStubCopyWithoutTheNetwork(t *testing.T) {
	p := goldenPalette(t)
	p.updater = update.NoUpdates{Version: "2.0.0-go"}
	answer := make(chan string, 1)
	p.checkForUpdates(func(msg string) { answer <- msg })
	select {
	case msg := <-answer:
		if want := "Updates are not available in this build yet"; msg != want {
			t.Fatalf("answer = %q, want %q", msg, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no answer for a check that contacts nothing")
	}
}
