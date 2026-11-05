package extwindow

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func noteView(id string) api.View {
	return api.View{ID: id, Kind: api.ViewList, Title: "Ticker " + id}
}

func boolPtr(v bool) *bool { return &v }

func TestCreateOpensVisibleWindowWithDefaultsOfTheOneXCompanion(t *testing.T) {
	opener := &fakeOpener{}
	m := newTestManager(opener, nil, fake.NewClock())
	if err := m.Create("ticker", noteView("main"), api.WindowOptions{}); err != nil {
		t.Fatal(err)
	}
	s := opener.surfaces[0]
	if !s.visible || s.shows != 1 {
		t.Fatalf("window visible=%v shows=%d, want shown once", s.visible, s.shows)
	}
	if s.spec.Title != "Ticker main" || s.spec.Width != 560 || s.spec.Height != 420 || !s.spec.AlwaysOnTop {
		t.Fatalf("spec = %+v, want the view title, 560 by 420 and always on top", s.spec)
	}
}

func TestLargeFloatingWindowOptionsReachTheSpec(t *testing.T) {
	opener := &fakeOpener{}
	m := newTestManager(opener, nil, fake.NewClock())
	options := api.WindowOptions{Title: "Note", Size: api.SizeLarge, AlwaysOnTop: boolPtr(false), TitleBar: "hidden", Chrome: "none", VisibleOnAllSpaces: true}
	if err := m.Create("ticker", noteView("note"), options); err != nil {
		t.Fatal(err)
	}
	want := Spec{Title: "Note", Width: 900, Height: 680, AlwaysOnTop: false, VisibleOnAllSpaces: true, TitleBarHidden: true, Frameless: true}
	if got := opener.surfaces[0].spec; got != want {
		t.Fatalf("spec = %+v, want %+v", got, want)
	}
}

func TestCreateWithOpenIDUpdatesTheWindowInPlace(t *testing.T) {
	opener := &fakeOpener{}
	m := newTestManager(opener, nil, fake.NewClock())
	_ = m.Create("ticker", noteView("main"), api.WindowOptions{})
	updated := noteView("main")
	updated.Title = "Renamed"
	if err := m.Create("ticker", updated, api.WindowOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(opener.surfaces) != 1 {
		t.Fatalf("opened %d windows, want the open one updated", len(opener.surfaces))
	}
	if opener.surfaces[0].root.Title != "Renamed" || opener.surfaces[0].shows != 2 {
		t.Fatalf("root title %q shows %d, want the new page shown again", opener.surfaces[0].root.Title, opener.surfaces[0].shows)
	}
}

func TestWindowsOfTwoExtensionsWithOneIDAreSeparate(t *testing.T) {
	opener := &fakeOpener{}
	m := newTestManager(opener, nil, fake.NewClock())
	_ = m.Create("one", noteView("main"), api.WindowOptions{})
	_ = m.Create("two", noteView("main"), api.WindowOptions{})
	if len(opener.surfaces) != 2 {
		t.Fatalf("opened %d windows, want one per extension", len(opener.surfaces))
	}
}

func TestShowHideAndToggleFollowTheVisibleState(t *testing.T) {
	opener := &fakeOpener{}
	m := newTestManager(opener, nil, fake.NewClock())
	ref := api.WindowRef{ID: "main"}
	_ = m.Create("ticker", noteView("main"), api.WindowOptions{})
	s := opener.surfaces[0]
	must(t, m.Hide("ticker", ref))
	if s.visible {
		t.Fatal("hide left the window visible")
	}
	must(t, m.Show("ticker", ref))
	must(t, m.Toggle("ticker", ref))
	if s.visible {
		t.Fatal("toggle of a visible window did not hide it")
	}
	must(t, m.Toggle("ticker", ref))
	if !s.visible {
		t.Fatal("toggle of a hidden window did not show it")
	}
}

func TestToggleCreatesTheWindowWhenItIsNotOpen(t *testing.T) {
	opener := &fakeOpener{}
	m := newTestManager(opener, nil, fake.NewClock())
	view := noteView("main")
	must(t, m.Toggle("ticker", api.WindowRef{View: &view}))
	if len(opener.surfaces) != 1 || !opener.surfaces[0].visible {
		t.Fatal("toggle with a view did not open and show the window")
	}
}

func TestOperationsOnAWindowThatIsNotOpenFail(t *testing.T) {
	m := newTestManager(&fakeOpener{}, nil, fake.NewClock())
	ref := api.WindowRef{ID: "missing"}
	for name, err := range map[string]error{
		"show":   m.Show("ticker", ref),
		"hide":   m.Hide("ticker", ref),
		"close":  m.Close("ticker", ref),
		"toggle": m.Toggle("ticker", ref),
	} {
		if !errors.Is(err, ErrNotOpen) {
			t.Fatalf("%s of a missing window = %v, want ErrNotOpen", name, err)
		}
	}
}

func TestClosingAWindowForgetsItsRecord(t *testing.T) {
	opener := &fakeOpener{}
	m := newTestManager(opener, nil, fake.NewClock())
	_ = m.Create("ticker", noteView("main"), api.WindowOptions{})
	must(t, m.Close("ticker", api.WindowRef{ID: "main"}))
	if err := m.Show("ticker", api.WindowRef{ID: "main"}); !errors.Is(err, ErrNotOpen) {
		t.Fatalf("show after close = %v, want ErrNotOpen", err)
	}
}

func TestHideOnBlurKeepsToggleClosedDuringTheGrace(t *testing.T) {
	clock := fake.NewClock()
	opener := &fakeOpener{}
	m := newTestManager(opener, nil, clock)
	_ = m.Create("ticker", noteView("main"), api.WindowOptions{HideOnBlur: true})
	s := opener.surfaces[0]
	s.blur()
	clock.Advance(100 * time.Millisecond)
	must(t, m.Toggle("ticker", api.WindowRef{ID: "main"}))
	if s.visible {
		t.Fatal("toggle within the grace after a blur reopened the window")
	}
	clock.Advance(400 * time.Millisecond)
	must(t, m.Toggle("ticker", api.WindowRef{ID: "main"}))
	if !s.visible {
		t.Fatal("toggle after the grace did not reopen the window")
	}
}

func TestBlurWithoutHideOnBlurLeavesTheWindowOpen(t *testing.T) {
	opener := &fakeOpener{}
	m := newTestManager(opener, nil, fake.NewClock())
	_ = m.Create("ticker", noteView("main"), api.WindowOptions{})
	opener.surfaces[0].blur()
	if !opener.surfaces[0].visible {
		t.Fatal("a window without hideOnBlur hid on blur")
	}
}

func TestPersistentWindowIsRestoredOnRelaunch(t *testing.T) {
	saver := &fakeSaver{}
	options := api.WindowOptions{Persistent: true, RestoreKey: "ticker.main", Title: "Ticker"}
	first := &fakeOpener{}
	must(t, newTestManager(first, saver, fake.NewClock()).Create("ticker", noteView("main"), options))
	if len(saver.state.Windows) != 1 {
		t.Fatalf("saved %d windows, want the persistent one", len(saver.state.Windows))
	}

	second := &fakeOpener{}
	ui := make(chan func(), 1)
	m := New(Deps{Open: second, Page: testPage, Saved: saver, Clock: fake.NewClock(), OnUI: func(fn func()) { ui <- fn }})
	m.RestorePersisted(func(extID, key string) (*api.WindowRestore, error) {
		fresh := api.WindowOptions{Title: "Restored"}
		return &api.WindowRestore{View: noteView("main"), Options: &fresh}, nil
	})
	(<-ui)()
	if len(second.surfaces) != 1 || second.surfaces[0].spec.Title != "Restored" || !second.surfaces[0].visible {
		t.Fatalf("restored windows = %+v, want one shown window titled by the fresh options", second.surfaces)
	}
}

func TestRestoreThatReturnsNothingPurgesTheSavedWindow(t *testing.T) {
	saver := &fakeSaver{}
	options := api.WindowOptions{Persistent: true, RestoreKey: "ticker.main"}
	must(t, newTestManager(&fakeOpener{}, saver, fake.NewClock()).Create("ticker", noteView("main"), options))
	ui := make(chan func(), 1)
	m := New(Deps{Open: &fakeOpener{}, Page: testPage, Saved: saver, OnUI: func(fn func()) { ui <- fn }})
	m.RestorePersisted(func(string, string) (*api.WindowRestore, error) { return nil, nil })
	(<-ui)()
	if len(saver.state.Windows) != 0 {
		t.Fatalf("saved windows after a nil restore = %d, want none", len(saver.state.Windows))
	}
}

func TestClosingByHandDropsThePersistentWindowButQuittingKeepsIt(t *testing.T) {
	saver := &fakeSaver{}
	options := api.WindowOptions{Persistent: true, RestoreKey: "ticker.main"}
	opener := &fakeOpener{}
	m := newTestManager(opener, saver, fake.NewClock())
	must(t, m.Create("ticker", noteView("main"), options))
	m.BeginQuit()
	must(t, m.Close("ticker", api.WindowRef{ID: "main"}))
	if len(saver.state.Windows) != 1 {
		t.Fatalf("saved windows after quit = %d, want the window kept", len(saver.state.Windows))
	}

	m2 := newTestManager(&fakeOpener{}, saver, fake.NewClock())
	must(t, m2.Create("ticker", noteView("main"), options))
	must(t, m2.Close("ticker", api.WindowRef{ID: "main"}))
	if len(saver.state.Windows) != 0 {
		t.Fatalf("saved windows after a hand close = %d, want none", len(saver.state.Windows))
	}
}

func TestRememberedFrameReopensWhereTheWindowWas(t *testing.T) {
	saver := &fakeSaver{}
	opener := &fakeOpener{}
	m := newTestManager(opener, saver, fake.NewClock())
	options := api.WindowOptions{RemembersFrame: true}
	must(t, m.Create("ticker", noteView("main"), options))
	opener.surfaces[0].move(Frame{X: 10, Y: 20, Width: 400, Height: 300})
	must(t, m.Close("ticker", api.WindowRef{ID: "main"}))

	must(t, m.Create("ticker", noteView("main"), options))
	saved := opener.surfaces[1].spec.Saved
	if saved == nil || *saved != (Frame{X: 10, Y: 20, Width: 400, Height: 300}) {
		t.Fatalf("reopened with frame %v, want the remembered one", saved)
	}
}

func TestPerformRunsTheDeclaredOperationAndRejectsUnknownOnes(t *testing.T) {
	opener := &fakeOpener{}
	m := newTestManager(opener, nil, fake.NewClock())
	view := noteView("main")
	must(t, m.Perform("ticker", api.Action{Kind: api.ActionWindow, Op: "create", Window: &api.WindowRef{View: &view}}))
	if len(opener.surfaces) != 1 {
		t.Fatal("create did not open a window")
	}
	if err := m.Perform("ticker", api.Action{Kind: api.ActionWindow, Op: "explode", Window: &api.WindowRef{ID: "main"}}); err == nil {
		t.Fatal("an unknown window op was accepted")
	}
}

func TestCreateRefusesOptionsTheHostCannotShow(t *testing.T) {
	m := newTestManager(&fakeOpener{}, nil, fake.NewClock())
	if err := m.Create("ticker", noteView("main"), api.WindowOptions{TitleBar: "neon"}); err == nil {
		t.Fatal("an invalid title bar was accepted")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
