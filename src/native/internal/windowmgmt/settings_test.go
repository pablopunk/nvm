package windowmgmt

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

const settingsDirectory = "/config"

func openSettingsDocument(t *testing.T, set *fake.Set) (*store.Store, *store.Collection[SettingsDocument]) {
	t.Helper()
	st, err := store.Open(set.Ports().FS, set.Ports().Clock, settingsDirectory)
	if err != nil {
		t.Fatal(err)
	}
	return st, store.Doc(st, "window-management.json", 1, DefaultSettingsDocument)
}

func TestSettingsWithoutADocumentStartAtDefaults(t *testing.T) {
	settings := NewSettingsStore(nil)
	if settings.Current() != geometry.DefaultSettings() {
		t.Fatalf("current = %+v, want defaults", settings.Current())
	}
}

func TestSettingsUpdatePersistsAcrossReopen(t *testing.T) {
	set := fake.NewSet()
	st, doc := openSettingsDocument(t, set)
	settings := NewSettingsStore(doc)
	if err := settings.Update(func(s *geometry.Settings) { s.InnerGap = 12 }); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, reopenedDoc := openSettingsDocument(t, set)
	defer reopened.Close()
	if got := NewSettingsStore(reopenedDoc).Current().InnerGap; got != 12 {
		t.Errorf("inner gap after reopen = %v, want 12", got)
	}
}

func TestSettingsResetRestoresDefaults(t *testing.T) {
	settings := NewSettingsStore(nil)
	_ = settings.Update(func(s *geometry.Settings) { s.OuterGap = 30 })
	if err := settings.Reset(); err != nil {
		t.Fatal(err)
	}
	if settings.Current().OuterGap != geometry.DefaultSettings().OuterGap {
		t.Errorf("outer gap = %v after reset", settings.Current().OuterGap)
	}
}
