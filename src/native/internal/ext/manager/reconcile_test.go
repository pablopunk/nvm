package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/portable"
	"github.com/pablopunk/nvm/native/internal/store"
)

// TestSyncedConfigChangeStartsAndStopsExtensionsWithoutRestart: a choice another machine synced into config.json is
// applied by ReconcileEnablement while the manager runs, and a choice withdrawn there stops the extension.
func TestSyncedConfigChangeStartsAndStopsExtensionsWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	ports := real.NewPorts()
	config := portable.Open(ports.FS, filepath.Join(dir, "config", "config.json"))
	open := func(data string) *enablement.Set {
		s, err := store.Open(ports.FS, ports.Clock, filepath.Join(dir, data))
		if err != nil {
			t.Fatalf("store.Open: %v", err)
		}
		t.Cleanup(func() { _ = s.Close() })
		return enablement.OpenSynced(s, config)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(Ports{Files: ports.FS, Clock: ports.Clock, Worker: real.NewWorker(), Processes: ports.Processes},
		filepath.Join(dir, "extensions"), open("linux-data"), Options{Executable: exe, DataRoot: filepath.Join(dir, "extension-data")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	h := &harness{t: t, m: m}
	if err := m.Install("acme.settings", fixtureContents(t, "acme.settings"), false); err != nil {
		t.Fatal(err)
	}
	if m.ReconcileEnablement() || m.List()[0].Enabled {
		t.Fatal("a new extension was enabled")
	}
	mac := open("mac-data")
	manifest, _, err := m.Definition("acme.settings")
	if err != nil {
		t.Fatal(err)
	}
	capabilities := reviewedCapabilities(manifest)
	if err := mac.SetEnabled("acme.settings", true, capabilities); err != nil {
		t.Fatal(err)
	}
	if !m.ReconcileEnablement() {
		t.Fatal("the synced enable was not applied")
	}
	h.waitRunning("acme.settings")
	if err := mac.SetEnabled("acme.settings", false, nil); err != nil {
		t.Fatal(err)
	}
	if !m.ReconcileEnablement() || m.List()[0].Enabled {
		t.Fatal("the synced disable was not applied")
	}
}
