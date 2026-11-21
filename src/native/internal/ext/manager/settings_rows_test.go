package manager

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestSettingsViewOfAFolderExtensionIsSavedThroughTheManager runs the settings fixture through a child process: its
// action returns a settings view, which the host validates, and the host reads and writes the declared settings the
// rows edit.
func TestSettingsViewOfAFolderExtensionIsSavedThroughTheManager(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.settings", fixtureContents(t, "acme.settings"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	h.waitRunning("acme.settings")

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	result, failure, err := h.m.Run(ctx, "acme.settings", api.Invocation{Ref: api.ActionRef{ID: "open"}})
	if err != nil || failure != nil {
		t.Fatalf("Run open: err %v failure %+v", err, failure)
	}
	if result.View == nil || result.View.Kind != api.ViewSettings || result.View.Settings == nil || len(result.View.Settings.Rows) != 5 {
		t.Fatalf("open answered %+v; want a settings view with five rows", result.View)
	}

	if value, err := h.m.SettingValue("acme.settings", "compact"); err != nil || string(value) != "false" {
		t.Fatalf("compact = %s, %v; want the declared default false", value, err)
	}
	if err := h.m.WriteSetting("acme.settings", "compact", json.RawMessage(`true`)); err != nil {
		t.Fatalf("WriteSetting: %v", err)
	}
	if value, err := h.m.SettingValue("acme.settings", "compact"); err != nil || string(value) != "true" {
		t.Fatalf("compact after write = %s, %v; want true", value, err)
	}
}
