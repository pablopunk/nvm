package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/aicommands"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/shortcuts/actioncli"
)

// A shortcut is bound only for an action the command line runs: the two lookups agree, for built-ins, window
// management, extension commands and ids that do not exist. Both go through the real manager of the golden palette.
func TestBoundActionsAreTheOnesRunResolves(t *testing.T) {
	p := goldenPalette(t)
	p.Attach(&testWindow{})
	ids := []string{
		shortcuts.PaletteID, shortcuts.PaletteToggleID, shortcuts.CommandKeyboardShortcuts, shortcuts.CommandQuickNotes,
		shortcuts.CommandFixSelectedText, "extension-action:window-management:layout.reasonable-size",
		"extension:" + aicommands.ID + ":" + aicommands.FixCommandID,
		"command:no-such-command", "extension:gone.example:run", "extension-action:window-management:layout.no-such", "unknown",
	}
	actions := map[string]string{}
	for _, id := range ids {
		actions[id] = "Command+Alt+K"
	}
	bound := p.runnableActions(actions)
	for _, id := range ids {
		_, isBound := bound[id]
		rejected := p.runActionCommandOnUI(id).Outcome == actioncli.Rejected
		if isBound == rejected {
			t.Errorf("%s: bound=%v but run rejected=%v", id, isBound, rejected)
		}
	}
	for _, id := range []string{"command:no-such-command", "extension:gone.example:run", "unknown"} {
		if _, ok := bound[id]; ok {
			t.Errorf("%s is bound though it does not exist", id)
		}
	}
	if _, ok := bound[shortcuts.CommandKeyboardShortcuts]; !ok {
		t.Error("a real built-in command is not bound")
	}
}

// A shortcut synced from another machine can open an app that is not installed here, such as a macOS app path on
// Linux. It is not bound, so the key never answers "not installed".
func TestAnAppShortcutIsBoundOnlyWhenTheAppExistsHere(t *testing.T) {
	p := goldenPalette(t)
	p.Attach(&testWindow{})
	present, missing := "/Applications/Here.app", "/Applications/Gone.app"
	if err := p.ports.FS.MkdirAll(present, 0o755); err != nil {
		t.Fatal(err)
	}
	bound := p.runnableActions(map[string]string{"app:" + present: "Command+Alt+P", "app:" + missing: "Command+Alt+G"})
	if _, ok := bound["app:"+missing]; ok {
		t.Error("an app that is not installed here was bound")
	}
	if _, ok := bound["app:"+present]; !ok {
		t.Error("an app that exists here was not bound")
	}
}
