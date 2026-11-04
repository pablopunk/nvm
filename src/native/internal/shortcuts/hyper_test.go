package shortcuts

import (
	"slices"
	"testing"
)

// Ported from shortcut-label.test.ts: a shortcut that starts with the Hyper key shows ✦ in its place.
func TestLabelShowsTheHyperKeyAsSparkle(t *testing.T) {
	darwin := Labeler{GOOS: "darwin", Hyper: "Command+Control+Alt"}
	if got := darwin.Label("Command+Control+Alt+D"); got != "✦D" {
		t.Errorf("darwin = %q, want ✦D", got)
	}
	linux := Labeler{GOOS: "linux", Hyper: "Command+Control+Alt"}
	if got := linux.Label("Command+Control+Alt+D"); got != "✦D" {
		t.Errorf("linux = %q, want ✦D", got)
	}
	if got := darwin.Label("Command+Control+D"); got != "⌘⌃D" {
		t.Errorf("a shortcut without all of Hyper = %q, want ⌘⌃D", got)
	}
	if got := darwin.Label("Command+Control"); got != "⌘⌃" {
		t.Errorf("a shortcut shorter than Hyper = %q, want ⌘⌃", got)
	}
}

func TestLabelWithoutAHyperKeyIsUnchanged(t *testing.T) {
	if got := (Labeler{GOOS: "darwin"}).Label("Command+Control+Alt+D"); got != "⌘⌃⌥D" {
		t.Errorf("no Hyper = %q, want ⌘⌃⌥D", got)
	}
	if got := (Labeler{GOOS: "darwin", Hyper: ""}).Label("Command+Alt+K"); got != "⌘⌥K" {
		t.Errorf("blank Hyper = %q, want ⌘⌥K", got)
	}
}

func TestHyperKeyOrderMatters(t *testing.T) {
	labeler := Labeler{GOOS: "darwin", Hyper: DefaultHyperKey}
	if got := labeler.Label("Control+Command+Alt+Shift+D"); got != "⌃⌘⌥⇧D" {
		t.Errorf("out-of-order modifiers = %q, want ⌃⌘⌥⇧D", got)
	}
	if got := labeler.Label("Command+Control+Alt+Shift+D"); got != "✦D" {
		t.Errorf("default Hyper = %q, want ✦D", got)
	}
}

func TestKeycapsStartWithTheHyperSparkle(t *testing.T) {
	labeler := Labeler{GOOS: "darwin", Hyper: "Command+Control+Alt"}
	if got := labeler.Keycaps("Command+Control+Alt+D"); !slices.Equal(got, []string{"✦", "D"}) {
		t.Errorf("hyper keycaps = %q", got)
	}
	if got := (Labeler{GOOS: "darwin"}).Keycaps("Command+Alt+K"); !slices.Equal(got, []string{"⌘", "⌥", "K"}) {
		t.Errorf("plain keycaps = %q", got)
	}
}

func TestStripHyperNeedsTheWholeKeyInOrder(t *testing.T) {
	if rest, ok := StripHyper("Command+Control+Alt+Shift+F5", DefaultHyperKey); !ok || rest != "F5" {
		t.Errorf("strip = %q, %v", rest, ok)
	}
	if _, ok := StripHyper("Command+Control+Alt+D", DefaultHyperKey); ok {
		t.Error("a partial Hyper key matched")
	}
	if _, ok := StripHyper("Command+Control+Alt+D", ""); ok {
		t.Error("a blank Hyper key matched")
	}
}

func TestModifiersAccelerator(t *testing.T) {
	mods := Modifiers{Command: true, Control: true}.Or(Modifiers{Alt: true, Shift: true})
	if got := ModifiersAccelerator(mods); got != DefaultHyperKey {
		t.Errorf("held modifiers = %q, want %q", got, DefaultHyperKey)
	}
	if got := ModifiersAccelerator(Modifiers{}); got != "" {
		t.Errorf("no modifiers = %q, want blank", got)
	}
}
