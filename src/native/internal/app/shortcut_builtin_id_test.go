package app

import (
	"testing"

	windowmanagementbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/windowmanagement"
)

// TestWindowShortcutKeysNameTheBuiltInByItsId: a window command's 1.x shortcut key names the built-in by its short name, and
// the shortcut runs the built-in by its id, so the key splits to the id the manager runs.
func TestWindowShortcutKeysNameTheBuiltInByItsId(t *testing.T) {
	extID, commandID, ok := splitExtensionCommandShortcutID("extension-action:window-management:layout.left-half")
	if !ok || extID != windowmanagementbuiltin.ID || commandID != "layout.left-half" {
		t.Fatalf("split = %q %q %v, want %q layout.left-half", extID, commandID, ok, windowmanagementbuiltin.ID)
	}
}

// TestOtherShortcutKeysKeepTheirExtensionName: a key naming an extension that is not a built-in splits as written.
func TestOtherShortcutKeysKeepTheirExtensionName(t *testing.T) {
	extID, commandID, ok := splitExtensionCommandShortcutID("extension-action:acme.notes:open")
	if !ok || extID != "acme.notes" || commandID != "open" {
		t.Fatalf("split = %q %q %v, want acme.notes open", extID, commandID, ok)
	}
}
