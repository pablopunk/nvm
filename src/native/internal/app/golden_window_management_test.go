package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/windowmanagement"
)

// windowManagementViewGoldens are the views of the Window Management built-in the goldens render, each with the action
// of the command that opens it.
var windowManagementViewGoldens = []struct {
	name     string
	actionID string
}{
	{name: "window-settings", actionID: "settings"},
	{name: "window-custom-commands", actionID: "custom-commands"},
}

// TestWindowManagementViewGoldens renders the views the Window Management built-in opens from its settings command and its
// Custom Window Commands command, as the palette draws them.
func TestWindowManagementViewGoldens(t *testing.T) {
	for _, view := range windowManagementViewGoldens {
		p := goldenPalette(t)
		result, ok := p.builtInRun(windowmanagement.ID, view.actionID)
		if !ok || result.View == nil {
			t.Fatalf("%s: the Window Management built-in opened no view", view.actionID)
		}
		goldenOpen(p, p.builtInPalette(windowmanagement.ID, *result.View))
		golden.AssertMatrix(t, view.name, p.View, paletteGoldens)
	}
}
