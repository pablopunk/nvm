package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/apps"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// TestMovedQuitCommandsKeepTheirNativeShortcutKeys: the built-in Force Quit Apps and Quit All Apps are keyed by the IDs
// their native commands had, so a shortcut the user set on either still opens it, and it is named by its command.
func TestMovedQuitCommandsKeepTheirNativeShortcutKeys(t *testing.T) {
	cases := map[string]string{
		apps.ForceQuitCommandID: shortcuts.CommandForceQuitApps,
		apps.QuitAllCommandID:   shortcuts.CommandQuitAllApps,
	}
	for commandID, key := range cases {
		if got := extensionCommandShortcutID(apps.ID, commandID); got != key {
			t.Fatalf("shortcut key of %s = %q, want %q", commandID, got, key)
		}
		extID, gotCommand, ok := splitExtensionCommandShortcutID(key)
		if !ok || extID != apps.ID || gotCommand != commandID {
			t.Fatalf("split %q = %q, %q, %v; want %s, %s", key, extID, gotCommand, ok, apps.ID, commandID)
		}
	}
}

// TestOtherBuiltInCommandsKeepTheirExtensionKeys: only the moved commands change key; every other command keeps the
// extension key the manager builds.
func TestOtherBuiltInCommandsKeepTheirExtensionKeys(t *testing.T) {
	if got := extensionCommandShortcutID(apps.ID, "something-else"); got != "extension:nevermind.apps:something-else" {
		t.Fatalf("key = %q", got)
	}
}
