package app

import "testing"

// TestVisibleCommandsHaveUniqueStableIDs guards shortcut bindings: a visible command without an ID cannot take a
// shortcut, and two commands with one ID would share a binding.
func TestVisibleCommandsHaveUniqueStableIDs(t *testing.T) {
	p := goldenPalette(t)
	titles := map[string]string{}
	for _, cmd := range p.commands() {
		if cmd.Hidden {
			continue
		}
		if cmd.ID == "" {
			t.Errorf("visible command %q has no stable ID", cmd.Title)
			continue
		}
		if other, dup := titles[cmd.ID]; dup {
			t.Errorf("commands %q and %q share the ID %s", other, cmd.Title, cmd.ID)
		}
		titles[cmd.ID] = cmd.Title
	}
}
