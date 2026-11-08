package manager

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

func TestToggleSettingActionEnforcesAuthorityAndInstallation(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.settings", fixtureContents(t, "acme.settings"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.settings")
	identity := h.m.SnapshotIdentity("acme.settings")
	if err := h.m.ToggleSettingAction("acme.settings", "compact", identity); err != nil {
		t.Fatal(err)
	}
	if value, err := h.m.SettingValue("acme.settings", "compact"); err != nil || string(value) != "true" {
		t.Fatalf("value %s %v", value, err)
	}
	if err := h.m.ToggleSettingAction("acme.settings", "units", identity); err == nil || !strings.Contains(err.Error(), "boolean") {
		t.Fatalf("string toggle: %v", err)
	}
	if err := h.m.ToggleSettingAction("acme.settings", "missing", identity); err == nil {
		t.Fatal("undeclared setting accepted")
	}
	if err := h.m.ToggleSettingAction("acme.settings", "compact", []string{"untrusted"}); err == nil {
		t.Fatal("untrusted identity accepted")
	}
	if err := h.m.Disable("acme.settings"); err != nil {
		t.Fatal(err)
	}
	if err := h.m.ToggleSettingAction("acme.settings", "compact", identity); err == nil {
		t.Fatal("disabled extension accepted")
	}
	if err := h.m.Enable("acme.settings"); err != nil {
		t.Fatal(err)
	}
	if err := h.m.ToggleSettingAction("acme.settings", "compact", identity); err == nil {
		t.Fatal("stale installation accepted")
	}
}

func TestToggleSettingActionRequiresSettingsWrite(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.settings")
	contents[bundle.ManifestFile] = []byte(`{"apiVersion":"1.0","id":"acme.settings","title":"Settings","settings":[{"id":"compact","type":"boolean","value":false}]}`)
	if err := h.m.Install("acme.settings", contents, true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.settings")
	if err := h.m.ToggleSettingAction("acme.settings", "compact", h.m.SnapshotIdentity("acme.settings")); err == nil || !strings.Contains(err.Error(), "settings.write") {
		t.Fatalf("missing permission: %v", err)
	}
	if value, err := h.m.SettingValue("acme.settings", "compact"); err != nil || string(value) != "false" {
		t.Fatalf("denied toggle changed value: %s %v", value, err)
	}
}

func TestToggleSettingActionCannotBorrowAnotherExtensionIdentity(t *testing.T) {
	h := newHarness(t)
	first := fixtureContents(t, "acme.settings")
	second := fixtureContents(t, "acme.settings")
	second[bundle.ManifestFile] = []byte(strings.Replace(string(second[bundle.ManifestFile]), `"acme.settings"`, `"acme.other"`, 1))
	if err := h.m.Install("acme.settings", first, true); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Install("acme.other", second, true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.settings")
	h.waitRunning("acme.other")
	identity := h.m.SnapshotIdentity("acme.settings")
	if err := h.m.ToggleSettingAction("acme.other", "compact", identity); err == nil {
		t.Fatal("another extension's identity was accepted")
	}
	if value, err := h.m.SettingValue("acme.other", "compact"); err != nil || string(value) != "false" {
		t.Fatalf("other extension changed: %s %v", value, err)
	}
}

func TestToggleSettingRejectsReinstalledOrigin(t *testing.T) {
	h := newHarness(t)
	const id = "acme.settings"
	contents := fixtureContents(t, id)
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	first := h.m.SnapshotIdentity(id)
	if err := h.m.Uninstall(id); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	if err := h.m.ToggleSettingAction(id, "compact", first); err == nil {
		t.Fatal("old installation toggled replacement")
	}
}

func TestSettingWriteRejectsReinstalledOrigin(t *testing.T) {
	h := newHarness(t)
	const id = "acme.settings"
	contents := fixtureContents(t, id)
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	first := h.m.SnapshotIdentity(id)
	if err := h.m.Uninstall(id); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	if err := h.m.WriteSettingForIdentity(id, "compact", json.RawMessage("true"), first); err == nil {
		t.Fatal("old installation wrote replacement settings")
	}
	if _, err := h.m.SettingValueForIdentity(id, "compact", first); err == nil {
		t.Fatal("old installation read replacement settings")
	}
}

func TestBoundSettingWriteStillRequiresCapability(t *testing.T) {
	h := newHarness(t)
	contents := fixtureContents(t, "acme.settings")
	contents[bundle.ManifestFile] = []byte(`{"apiVersion":"1.0","id":"acme.settings","title":"Settings","settings":[{"id":"compact","type":"boolean","value":false}]}`)
	if err := h.m.Install("acme.settings", contents, true); err != nil {
		t.Fatal(err)
	}
	identity := h.m.SnapshotIdentity("acme.settings")
	err := h.m.WriteSettingForIdentity("acme.settings", "compact", json.RawMessage("true"), identity)
	if err == nil || !strings.Contains(err.Error(), "settings.write") {
		t.Fatalf("missing capability: %v", err)
	}
}
