package extwindow

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

type testAuthority struct {
	identity any
	valid    bool
}

func (a *testAuthority) Identity() any { return a.identity }
func (a *testAuthority) Valid() bool   { return a.valid }

func TestBoundWindowRejectsOldInstallationAndBindsChild(t *testing.T) {
	old, current := &testAuthority{identity: new(int), valid: true}, &testAuthority{identity: new(int), valid: true}
	opener := &fakeOpener{}
	var childIdentity any
	m := New(Deps{Open: opener, Clock: fake.NewClock(), PageBound: func(_ string, identity any, v api.View, _ search.Host) *palette.View {
		childIdentity = identity
		return &palette.View{Title: v.Title}
	}})
	view := api.ListView("fixture")
	options := api.WindowOptions{ID: "main"}
	action := api.Action{Op: "create", Window: &api.WindowRef{View: &view, Options: &options}}
	if err := m.PerformBound("test", old, action); err != nil {
		t.Fatal(err)
	}
	if childIdentity != old.identity {
		t.Fatal("child did not inherit old installation")
	}
	old.valid = false
	if err := m.PerformBound("test", current, action); err != nil {
		t.Fatal(err)
	}
	if childIdentity != current.identity {
		t.Fatal("replacement child inherited old identity")
	}
	if err := m.PerformBound("test", old, api.Action{Op: "close", Window: &api.WindowRef{ID: "main"}}); !errors.Is(err, ErrExpired) {
		t.Fatalf("stale close=%v", err)
	}
	if len(opener.surfaces) != 2 || !opener.surfaces[1].visible {
		t.Fatal("stale operation affected replacement")
	}
}

func TestBoundWindowRevocationDuringPageCreation(t *testing.T) {
	authority := &testAuthority{identity: new(int), valid: true}
	opener := &fakeOpener{}
	m := New(Deps{Open: opener, Clock: fake.NewClock(), PageBound: func(_ string, _ any, v api.View, _ search.Host) *palette.View {
		authority.valid = false
		return &palette.View{Title: v.Title}
	}})
	view := api.ListView("fixture")
	err := m.PerformBound("test", authority, api.Action{Op: "create", Window: &api.WindowRef{View: &view}})
	if !errors.Is(err, ErrExpired) || len(opener.surfaces) != 1 || opener.surfaces[0].visible || len(m.windows) != 0 {
		t.Fatalf("err=%v windows=%d", err, len(m.windows))
	}
}

func TestRevokedRestorePreservesSavedWindow(t *testing.T) {
	authority := &testAuthority{identity: new(int), valid: false}
	saved := SavedWindow{ExtensionID: "test", RestoreKey: "saved"}
	saver := &fakeSaver{state: Saved{Windows: []SavedWindow{saved}}}
	opener := &fakeOpener{}
	m := New(Deps{Open: opener, Saved: saver, Clock: fake.NewClock()})
	m.adoptRestoredBound(saved, &api.WindowRestore{View: api.ListView("old")}, authority, nil)
	if len(saver.state.Windows) != 1 || len(opener.surfaces) != 0 {
		t.Fatal("revoked restore changed stored window")
	}
}

func TestRestoreRevokedDuringPageConstructionKeepsSavedState(t *testing.T) {
	authority := &testAuthority{identity: new(int), valid: true}
	saved := SavedWindow{ExtensionID: "test", RestoreKey: "saved"}
	saver := &fakeSaver{state: Saved{Windows: []SavedWindow{saved}}}
	opener := &fakeOpener{}
	m := New(Deps{Open: opener, Saved: saver, Clock: fake.NewClock(), PageBound: func(_ string, _ any, v api.View, _ search.Host) *palette.View {
		authority.valid = false
		return &palette.View{Title: v.Title}
	}})
	m.adoptRestoredBound(saved, &api.WindowRestore{View: api.ListView("old")}, authority, nil)
	if len(saver.state.Windows) != 1 || len(m.windows) != 0 || len(opener.surfaces) != 1 || opener.surfaces[0].visible {
		t.Fatalf("saved=%v windows=%d surfaces=%d", saver.state.Windows, len(m.windows), len(opener.surfaces))
	}
}

func TestOldWindowCloseCannotForgetReplacementRestore(t *testing.T) {
	old, current := &testAuthority{identity: new(int), valid: true}, &testAuthority{identity: new(int), valid: true}
	opener := &fakeOpener{}
	saver := &fakeSaver{}
	m := New(Deps{Open: opener, Saved: saver, Clock: fake.NewClock(), PageBound: func(_ string, _ any, v api.View, _ search.Host) *palette.View { return &palette.View{Title: v.Title} }})
	view := api.ListView("fixture")
	options := api.WindowOptions{ID: "main", RestoreKey: "saved", Persistent: true}
	action := api.Action{Op: "create", Window: &api.WindowRef{View: &view, Options: &options}}
	if err := m.PerformBound("test", old, action); err != nil {
		t.Fatal(err)
	}
	old.valid = false
	if err := m.PerformBound("test", current, action); err != nil {
		t.Fatal(err)
	}
	opener.surfaces[0].Close()
	if len(saver.state.Windows) != 1 || saver.state.Windows[0].RestoreKey != "saved" {
		t.Fatalf("saved windows=%v", saver.state.Windows)
	}
}
