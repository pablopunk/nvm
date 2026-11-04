package palette

import "testing"

// TestShortcutLabelSpellsSymbols checks the stored accelerators read as the footers spell them. The primary
// modifier follows KeyLabel, so the expected value is built with it.
func TestShortcutLabelSpellsSymbols(t *testing.T) {
	primary := KeyLabel("⌃")
	cases := []struct {
		accelerator string
		want        string
	}{
		{"Alt+Shift+Space", "⌥⇧Space"},
		{"Command+Alt+K", primary + "⌥K"},
		{"CommandOrControl+Shift+enter", primary + "⇧↵"},
		{"Control+Backspace", "⌃⌫"},
		{"Ctrl+v", "⌃V"},
		{"Escape", "Esc"},
		{"F5", "F5"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ShortcutLabel(tc.accelerator); got != tc.want {
			t.Errorf("ShortcutLabel(%q) = %q, want %q", tc.accelerator, got, tc.want)
		}
	}
}

// TestShortcutLabelControlStaysControlOnMacOS checks Control is ⌃ on every platform: only the primary modifier is
// spelled ⌘ on macOS.
func TestShortcutLabelControlStaysControlOnMacOS(t *testing.T) {
	if got := ShortcutLabel("Control+K"); got != "⌃K" {
		t.Errorf("ShortcutLabel(Control+K) = %q, want ⌃K", got)
	}
}
