package enablement

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/portable"
	"github.com/pablopunk/nvm/native/internal/store"
)

const sharedConfig = "/home/me/.config/nevermind/config.json"

// machine opens a fresh data directory over files, the way another computer sharing only the config folder would.
func machine(t *testing.T, files *fake.FS, dataDir string) (*Set, *store.Store) {
	t.Helper()
	s, err := store.Open(files, fake.NewClock(), dataDir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return OpenSynced(s, portable.Open(files, sharedConfig)), s
}

// TestEnabledOnOneMachineIsEnabledOnAnotherWithAFreshDataDirectory: the choice lives in the synced config, so a
// machine that has only the config file sees the extension enabled.
func TestEnabledOnOneMachineIsEnabledOnAnotherWithAFreshDataDirectory(t *testing.T) {
	files := fake.NewFS()
	mac, macStore := machine(t, files, "/mac/data")
	defer macStore.Close()
	caps := []string{"network", "files"}
	if err := mac.SetEnabled("acme.github", true, caps); err != nil {
		t.Fatal(err)
	}
	linux, linuxStore := machine(t, files, "/linux/data")
	defer linuxStore.Close()
	if on, err := linux.Enabled("acme.github", caps); err != nil || !on {
		t.Fatalf("Enabled on the second machine = %v, %v; want true", on, err)
	}
	if err := linux.SetEnabled("acme.github", false, caps); err != nil {
		t.Fatal(err)
	}
	if on, _ := mac.Enabled("acme.github", caps); on {
		t.Fatal("a disable on one machine did not reach the other")
	}
}

// TestAnExtensionNobodyEnabledStaysDisabled: the safety default holds with a synced file full of other choices.
func TestAnExtensionNobodyEnabledStaysDisabled(t *testing.T) {
	files := fake.NewFS()
	mac, macStore := machine(t, files, "/mac/data")
	defer macStore.Close()
	if err := mac.SetEnabled("acme.github", true, nil); err != nil {
		t.Fatal(err)
	}
	linux, linuxStore := machine(t, files, "/linux/data")
	defer linuxStore.Close()
	if on, _ := linux.Enabled("acme.camera", nil); on {
		t.Fatal("a new extension started enabled")
	}
}

// TestCapabilitiesThatGrewAskAgain: an extension that declares more than was reviewed is off until enabled again;
// one that declares less stays on.
func TestCapabilitiesThatGrewAskAgain(t *testing.T) {
	set, s := machine(t, fake.NewFS(), "/data")
	defer s.Close()
	if err := set.SetEnabled("acme.camera", true, []string{"camera"}); err != nil {
		t.Fatal(err)
	}
	if on, _ := set.Enabled("acme.camera", []string{"camera", "network"}); on {
		t.Fatal("an extension whose capabilities grew stayed enabled")
	}
	if on, _ := set.Enabled("acme.camera", nil); !on {
		t.Fatal("an extension that asks for less lost its enabled state")
	}
	if err := set.SetEnabled("acme.camera", true, []string{"camera", "network"}); err != nil {
		t.Fatal(err)
	}
	if on, _ := set.Enabled("acme.camera", []string{"camera", "network"}); !on {
		t.Fatal("reviewing again did not enable the extension")
	}
}

// TestLocalEnablementMovesToTheSyncedConfigOnce: the old local document is read, its choices are pinned to the
// capabilities declared now, and a later disable is not undone by the stale local entry.
func TestLocalEnablementMovesToTheSyncedConfigOnce(t *testing.T) {
	files := fake.NewFS()
	local, s := openSet(t, files)
	if err := local.SetEnabled("acme.github", true, nil); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	upgraded, s2 := openSyncedAt(t, files, "/cfg/nvm")
	if on, _ := upgraded.Enabled("acme.github", []string{"network"}); !on {
		t.Fatal("the local choice was lost by the upgrade")
	}
	if err := upgraded.SetEnabled("acme.github", false, nil); err != nil {
		t.Fatal(err)
	}
	_ = s2.Close()
	again, s3 := openSyncedAt(t, files, "/cfg/nvm")
	defer s3.Close()
	if on, _ := again.Enabled("acme.github", []string{"network"}); on {
		t.Fatal("the stale local entry turned a disabled extension back on")
	}
	other, s4 := machine(t, files, "/other/data")
	defer s4.Close()
	if on, _ := other.Enabled("acme.github", nil); on {
		t.Fatal("a disabled extension is enabled on another machine")
	}
}

// TestMigratedChoiceReachesAnotherMachine: after migration the synced file carries the choice.
func TestMigratedChoiceReachesAnotherMachine(t *testing.T) {
	files := fake.NewFS()
	local, s := openSet(t, files)
	_ = local.SetEnabled("acme.github", true, nil)
	_ = s.Close()
	upgraded, s2 := openSyncedAt(t, files, "/cfg/nvm")
	defer s2.Close()
	if on, _ := upgraded.Enabled("acme.github", []string{"network"}); !on {
		t.Fatal("not migrated")
	}
	other, s3 := machine(t, files, "/other/data")
	defer s3.Close()
	if on, _ := other.Enabled("acme.github", []string{"network"}); !on {
		t.Fatal("the migrated choice did not reach the synced file")
	}
}

func openSyncedAt(t *testing.T, files *fake.FS, dir string) (*Set, *store.Store) {
	t.Helper()
	return machine(t, files, dir)
}

// TestConfigChangedOnDiskIsSeenWithoutReopening: the synced file is read on every check.
func TestConfigChangedOnDiskIsSeenWithoutReopening(t *testing.T) {
	files := fake.NewFS()
	linux, ls := machine(t, files, "/linux/data")
	defer ls.Close()
	if on, _ := linux.Enabled("acme.github", nil); on {
		t.Fatal("enabled before any choice")
	}
	mac, ms := machine(t, files, "/mac/data")
	defer ms.Close()
	_ = mac.SetEnabled("acme.github", true, nil)
	if on, _ := linux.Enabled("acme.github", nil); !on {
		t.Fatal("a config change on disk was not picked up")
	}
}
