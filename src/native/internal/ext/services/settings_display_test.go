package services

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func publishingHandler(t *testing.T, displays *SharedDisplays) *Handler {
	t.Helper()
	r := newRig(t)
	ports := real.NewPorts()
	manifest := api.Manifest{
		APIVersion: "1.0",
		Identity:   api.Identity{ID: "nevermind.dictation", Title: "Dictation"},
		Settings:   []api.SettingDefinition{{ID: "device", Type: api.SettingString}},
		Publishes:  []string{"device"},
	}
	deps := Deps{Files: ports.FS, Clock: r.clock, DataRoot: r.root, Keychain: r.keychain, Network: r.network, Clipboard: r.clipboard, Logs: r.logs, Displays: displays}
	h := New(deps, manifest)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

// TestBuiltInPublishesTheDisplayOfItsListedSettingOnly checks that a publish is stored under the publisher's id, and
// that a built-in cannot publish the display of a setting its manifest does not list in Publishes.
func TestBuiltInPublishesTheDisplayOfItsListedSettingOnly(t *testing.T) {
	displays := NewSharedDisplays()
	h := publishingHandler(t, displays)

	mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpPublishDisplay, protocol.KeyValueArgs{Key: "device", Value: []byte(`"USB Mic"`)}))
	text, ok := displays.Read(api.SettingRef{Extension: "nevermind.dictation", Setting: "device"})
	if !ok || text != "USB Mic" {
		t.Fatalf("published display = %q, %v; want USB Mic", text, ok)
	}
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpPublishDisplay, protocol.KeyValueArgs{Key: "volume", Value: []byte(`"loud"`)}), api.CodeNotFound, "publishes no display")
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpPublishDisplay, protocol.KeyValueArgs{Key: "device", Value: []byte(`3`)}), api.CodeInvalid, "not text")
}

func TestPublishingWithoutAStoreIsUnavailable(t *testing.T) {
	h := publishingHandler(t, nil)
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpPublishDisplay, protocol.KeyValueArgs{Key: "device", Value: []byte(`"USB Mic"`)}), api.CodeUnavailable, "not kept here")
}
