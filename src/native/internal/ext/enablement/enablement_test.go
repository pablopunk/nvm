package enablement

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

func openSet(t *testing.T, files *fake.FS) (*Set, *store.Store) {
	t.Helper()
	s, err := store.Open(files, fake.NewClock(), "/cfg/nvm")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	return Open(s), s
}

func TestDroppedInExtensionsDefaultToDisabled(t *testing.T) {
	set, s := openSet(t, fake.NewFS())
	defer s.Close()
	enabled, err := set.Enabled("acme.good")
	if err != nil || enabled {
		t.Fatalf("Enabled = %v, %v; want false, nil", enabled, err)
	}
}

func TestEnableAndDisablePersistAcrossReopen(t *testing.T) {
	files := fake.NewFS()
	set, s := openSet(t, files)
	if err := set.SetEnabled("acme.good", true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, s2 := openSet(t, files)
	defer s2.Close()
	if enabled, _ := reopened.Enabled("acme.good"); !enabled {
		t.Fatal("enabled state was lost on reopen")
	}
	if err := reopened.SetEnabled("acme.good", false); err != nil {
		t.Fatalf("SetEnabled off: %v", err)
	}
	if enabled, _ := reopened.Enabled("acme.good"); enabled {
		t.Fatal("disable did not take effect")
	}
}
