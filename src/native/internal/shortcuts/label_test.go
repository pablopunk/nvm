package shortcuts

import "testing"

// Ported from shortcut-label.test.ts: "shortcutLabel uses native key symbols on macOS".
func TestLabelUsesNativeKeySymbolsOnMacOS(t *testing.T) {
	assertLabel(t, "Command+Backspace", "darwin", "⌘⌫")
	assertLabel(t, "Alt+Space", "darwin", "⌥Space")
	assertLabel(t, "Command+Alt+K", "darwin", "⌘⌥K")
}

// Ported from shortcut-label.test.ts: "shortcutLabel uses platform key names on Linux and Windows".
func TestLabelUsesPlatformKeyNamesOffMacOS(t *testing.T) {
	assertLabel(t, "Alt+Space", "linux", "Alt+Space")
	assertLabel(t, "Alt+Space", "windows", "Alt+Space")
	assertLabel(t, "Control+Alt+K", "linux", "Control+Alt+K")
	assertLabel(t, "Control+Alt+K", "windows", "Control+Alt+K")
}

// Ported from shortcut-label.test.ts: "shortcutLabel names the meta key for Linux and Windows instead of Command".
func TestLabelNamesTheMetaKeyOffMacOS(t *testing.T) {
	assertLabel(t, "Command+Y", "linux", "Super+Y")
	assertLabel(t, "Command+Shift+Enter", "linux", "Super+Shift+Enter")
	assertLabel(t, "Command+Backspace", "windows", "Win+Backspace")
}

// Ported from shortcut-label.test.ts: "CommandOrControl shows the primary modifier for each platform".
func TestLabelShowsCommandOrControlPerPlatform(t *testing.T) {
	assertLabel(t, "CommandOrControl+Enter", "darwin", "⌘↵")
	assertLabel(t, "CommandOrControl+Enter", "linux", "Control+Enter")
	assertLabel(t, "CommandOrControl+Enter", "windows", "Control+Enter")
}

// Ported from shortcut-utils.test.ts: "shortcut display uses native macOS symbols only on macOS".
func TestLabelAcceptsAnyWordingOfTheSameShortcut(t *testing.T) {
	assertLabel(t, "ctrl+option+k", "windows", "Control+Alt+K")
	assertLabel(t, "Command+Alt+K", "linux", "Super+Alt+K")
	assertLabel(t, "cmd+option+k", "darwin", "⌘⌥K")
}

func assertLabel(t *testing.T, accelerator, goos, want string) {
	t.Helper()
	if got := Label(accelerator, goos); got != want {
		t.Errorf("Label(%q, %s) = %q, want %q", accelerator, goos, got, want)
	}
}

func TestSpotlightOwnsCommandSpaceOnlyOnMacOS(t *testing.T) {
	if got := SystemReserved("Command+Space", "darwin"); got != "Spotlight" {
		t.Errorf("darwin owner = %q, want Spotlight", got)
	}
	if got := SystemReserved("cmd+space", "darwin"); got != "Spotlight" {
		t.Errorf("darwin owner for a wording of Command+Space = %q, want Spotlight", got)
	}
	if got := SystemReserved("Command+Space", "linux"); got != "" {
		t.Errorf("linux owner = %q, want none", got)
	}
}
