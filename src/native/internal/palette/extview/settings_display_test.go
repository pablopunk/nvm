package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestSharedDisplayRowShowsTheTextAnotherBuiltInPublishes checks that a row showing a shared setting takes the published
// text, and its own accessory until something is published.
func TestSharedDisplayRowShowsTheTextAnotherBuiltInPublishes(t *testing.T) {
	shown := api.SettingRef{Extension: "nevermind.dictation", Setting: "device"}
	open := api.Action{Kind: api.ActionOpenURL, URL: "https://example.com"}
	view := api.SettingsView("Settings", api.SettingRow{ID: "dictation", Kind: api.SettingRowAction, Title: "Dictation", Accessory: "fallback",
		Shows: &shown, Action: &open})
	published := ""
	deps := Deps{ReadDisplay: func(reader string, ref api.SettingRef) (string, bool) {
		if reader != "nevermind.settings" || ref != shown || published == "" {
			return "", false
		}
		return published, true
	}}
	session := NewSession(deps, "nevermind.settings", view)

	if row := settingsRow(t, session.Palette(), "", "Dictation"); row.Accessory != "fallback" {
		t.Fatalf("accessory %q before a publish; want the row's own", row.Accessory)
	}
	published = "USB Mic"
	if row := settingsRow(t, session.Palette(), "", "Dictation"); row.Accessory != "USB Mic" {
		t.Fatalf("accessory %q after a publish; want the published microphone", row.Accessory)
	}
}
