package extview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

func TestShortcutLabelShowsTheHyperKeyAsSparkle(t *testing.T) {
	if got := shortcutLabel("Command+Control+Alt+D", "Command+Control+Alt"); got != "✦D" {
		t.Fatalf("hyper label = %q, want ✦D", got)
	}
	if got := shortcutLabel("Command+Control+Alt+D", ""); got == "✦D" {
		t.Fatalf("a label without a hyper key showed ✦: %q", got)
	}
}

// TestSessionActionKeepsTheRealModifiersUnderHyper checks that the key the palette matches is the accelerator's own
// modifiers, while the label shows the hyper sparkle.
func TestSessionActionKeepsTheRealModifiersUnderHyper(t *testing.T) {
	session := &Session{deps: Deps{HyperKey: func() string { return "Command+Control+Alt" }}}
	action := session.withShortcut(search.Action{}, "Command+Control+Alt+D")
	if action.Mods != ui.Cmd|ui.Ctrl|ui.Alt || action.Key != ui.KeyD {
		t.Fatalf("matching keys = %v %v, want the accelerator's modifiers and D", action.Mods, action.Key)
	}
	if action.Label != "✦D" {
		t.Fatalf("label = %q, want ✦D", action.Label)
	}
}
