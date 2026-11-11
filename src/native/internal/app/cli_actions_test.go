package app

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/shortcuts/actioncli"
)

// The command line runs an action as its shortcut does and never runs a substitute for an unknown one.
func TestActionCommandOutcomes(t *testing.T) {
	p := goldenPalette(t)
	p.Attach(&testWindow{})
	if r := p.runActionCommandOnUI(shortcuts.PaletteID); r.Outcome != actioncli.Opened || !p.Visible() {
		t.Fatalf("palette: %+v visible=%v", r, p.Visible())
	}
	if r := p.runActionCommandOnUI(shortcuts.CommandKeyboardShortcuts); r.Outcome != actioncli.Opened {
		t.Fatalf("keyboard shortcuts: %+v", r)
	}
	for _, id := range []string{"command:no-such-command", "extension:gone.example:run", "unknown"} {
		r := p.runActionCommandOnUI(id)
		if r.Outcome != actioncli.Rejected || r.Message == "" {
			t.Errorf("%s: %+v", id, r)
		}
	}
}

func TestPaletteRowCopiesTheToggleCommand(t *testing.T) {
	if got := actionCommandLine(shortcuts.PaletteID); !strings.HasSuffix(got, " run palette-toggle") {
		t.Fatalf("palette command line %q", got)
	}
	if got := actionCommandLine(shortcuts.CommandQuickNotes); !strings.HasSuffix(got, " run command:quick-notes") {
		t.Fatalf("quick notes command line %q", got)
	}
}
