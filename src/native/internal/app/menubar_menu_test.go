package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/update"
	updatefake "github.com/pablopunk/nvm/native/internal/update/fake"
)

// uiWindow is the palette's window with its own UI thread: Update queues fn for one goroutine, as the platform's event
// loop runs it, so background updates and clicks never touch the palette at the same time.
type uiWindow struct {
	*stubWindow
	jobs chan func()
}

func (w uiWindow) Update(fn func()) { w.jobs <- fn }

// menuUpdates is a wired app whose updater is the real update service over a scripted engine. The menu bar is the fake
// tray's item, drawn as macOS draws it. It records the drawn menu by the updater's status, so each state's menu is
// checked as the app drew it.
type menuUpdates struct {
	app    *App
	set    *fake.Set
	tray   *fake.Tray
	window *stubWindow
	ui     uiWindow
	engine *updatefake.Engine
	icon   []byte

	mu      sync.Mutex
	changes int
	menu    map[update.Status][]platform.MenuItem
}

func newMenuUpdates(t *testing.T, engine *updatefake.Engine) *menuUpdates {
	t.Helper()
	p, set := wiredApp(t)
	p.menuOS = "darwin"
	m := &menuUpdates{app: p, set: set, window: &stubWindow{}, engine: engine, menu: map[update.Status][]platform.MenuItem{}}
	m.ui = uiWindow{stubWindow: m.window, jobs: make(chan func(), 1024)}
	go func() {
		for fn := range m.ui.jobs {
			fn()
		}
	}()
	p.Attach(m.ui)
	p.updates = update.NewService(update.Options{
		Engine: engine, Clock: set.Clock, Current: nativeClientVersion, OnChange: m.onChange,
	})
	p.updater = p.updates
	p.setUpMenubar()
	m.tray = set.Tray.Trays()[0]
	m.icon, _ = m.tray.Icon()
	m.onChange(p.updates.State())
	return m
}

// onChange redraws the menu on the UI thread, as the app does, and records what it drew.
func (m *menuUpdates) onChange(update.State) {
	m.app.onUI(func() {
		m.app.refreshUpdateSurfaces()
		m.record(m.app.updates.State().Status)
	})
}

// record keeps the menu drawn for status. The status is read after the redraw, so it is the one the rows show.
func (m *menuUpdates) record(status update.Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.changes++
	m.menu[status] = m.tray.Items()
}

// onUIThread runs fn on the UI thread and waits for it, as a click from the menu bar does.
func (m *menuUpdates) onUIThread(fn func()) {
	done := make(chan struct{})
	m.ui.jobs <- func() {
		fn()
		close(done)
	}
	<-done
}

// click chooses the menu row with id, on the UI thread.
func (m *menuUpdates) click(t *testing.T, id string) {
	t.Helper()
	var err error
	m.onUIThread(func() { err = m.tray.Invoke(id) })
	if err != nil {
		t.Fatalf("Invoke %s: %v", id, err)
	}
}

// topPage is the title of the palette's top page and whether the palette is on screen.
func (m *menuUpdates) topPage() (string, bool) {
	var title string
	m.onUIThread(func() { title = m.app.Top().Title })
	return title, m.window.shown
}

func (m *menuUpdates) drawnMenu(status update.Status) ([]platform.MenuItem, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items, ok := m.menu[status]
	return items, ok
}

func (m *menuUpdates) waitForStatus(t *testing.T, status update.Status) []platform.MenuItem {
	t.Helper()
	eventually(t, "the menu to be drawn for "+string(status), func() bool {
		_, ok := m.drawnMenu(status)
		return ok
	})
	items, _ := m.drawnMenu(status)
	return items
}

func (m *menuUpdates) waitForChanges(t *testing.T, n int) {
	t.Helper()
	eventually(t, "the menu to be drawn for each change", func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.changes >= n
	})
}

// eventually waits for ok, which the updater's background work makes true, and fails the test if it never is.
func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func menuLabelsOf(items []platform.MenuItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Separator {
			out = append(out, "-")
			continue
		}
		out = append(out, item.Label)
	}
	return out
}

func menuRowOf(t *testing.T, items []platform.MenuItem, id string) platform.MenuItem {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("no %q row in %q", id, menuLabelsOf(items))
	return platform.MenuItem{}
}

func updateRowID(label string) string {
	if label == "Check for Updates…" {
		return "check-updates"
	}
	return "update"
}

// assertOneXMenu checks a drawn menu: the 1.x rows and dividers, the update or check row in its place, the palette
// shortcut on Open, the download glyph on the update row, and an unchanged menu bar glyph.
func assertOneXMenu(t *testing.T, m *menuUpdates, items []platform.MenuItem, updateLabel string, disabled bool) {
	t.Helper()
	want := []string{"Open Nevermind", "-", updateLabel, "Settings…", "-", "Quit Nevermind"}
	if got := menuLabelsOf(items); !slices.Equal(got, want) {
		t.Fatalf("menu = %q, want %q", got, want)
	}
	row := menuRowOf(t, items, updateRowID(updateLabel))
	if row.Disabled != disabled {
		t.Fatalf("row %q disabled = %v, want %v", updateLabel, row.Disabled, disabled)
	}
	if shortcut := m.app.shortcutsManager().PaletteAccelerator(); menuRowOf(t, items, "open").Accelerator != shortcut {
		t.Fatalf("Open shows shortcut %q, want the palette shortcut %q", menuRowOf(t, items, "open").Accelerator, shortcut)
	}
	if updateRowID(updateLabel) == "update" && (row.Icon == nil || !row.Icon.Template) {
		t.Fatalf("the update row %q has no template download glyph on macOS", updateLabel)
	}
	if icon, _ := m.tray.Icon(); !slices.Equal(icon, m.icon) {
		t.Fatal("the menu bar glyph changed with the update state; 1.x keeps the icon style")
	}
}

// newOfferEngine answers every check with version 2.1.0 and downloads it with the scripted install.
func newOfferEngine(install updatefake.Install) *updatefake.Engine {
	return updatefake.NewEngine([]updatefake.Check{{Offer: update.Offer{Version: "2.1.0"}, Found: true}}, install)
}

func checkOnce(t *testing.T, m *menuUpdates) {
	t.Helper()
	m.app.updates.Check(context.Background())
}

func TestMenuBarShowsTheOneXRowForEachUpdateState(t *testing.T) {
	cases := []struct {
		name     string
		engine   func() *updatefake.Engine
		drive    func(t *testing.T, m *menuUpdates)
		status   update.Status
		label    string
		disabled bool
	}{
		{
			name:   "idle",
			engine: func() *updatefake.Engine { return updatefake.NewEngine(nil) },
			status: update.StatusIdle, label: "Check for Updates…",
		},
		{
			name:   "checking",
			engine: func() *updatefake.Engine { return updatefake.NewEngine(nil) },
			drive:  checkOnce,
			status: update.StatusChecking, label: "Check for Updates…",
		},
		{
			name:   "unsupported build",
			engine: updatefake.Disabled,
			status: update.StatusUnsupported, label: "Check for Updates…",
		},
		{
			name:   "up to date",
			engine: func() *updatefake.Engine { return updatefake.NewEngine([]updatefake.Check{{}}) },
			drive:  checkOnce,
			status: update.StatusCurrent, label: "Check for Updates…",
		},
		{
			name: "check failed",
			engine: func() *updatefake.Engine {
				return updatefake.NewEngine([]updatefake.Check{{Err: errors.New("offline")}})
			},
			drive:  checkOnce,
			status: update.StatusFailed, label: "Check for Updates…",
		},
		{
			name: "downloading",
			engine: func() *updatefake.Engine {
				return newOfferEngine(updatefake.Install{Progress: []int64{50}, Total: 100, Block: true})
			},
			drive:  checkOnce,
			status: update.StatusDownloading, label: "Download Nevermind 2.1.0",
		},
		{
			name:   "ready to install",
			engine: func() *updatefake.Engine { return newOfferEngine(updatefake.Install{}) },
			drive:  checkOnce,
			status: update.StatusDownloaded, label: "Install Nevermind 2.1.0",
		},
		{
			name:   "installing",
			engine: func() *updatefake.Engine { return newOfferEngine(updatefake.Install{}) },
			drive: func(t *testing.T, m *menuUpdates) {
				checkOnce(t, m)
				m.waitForStatus(t, update.StatusDownloaded)
				m.click(t, "update")
			},
			status: update.StatusInstalling, label: "Installing Nevermind 2.1.0", disabled: true,
		},
		{
			name:   "install failed",
			engine: func() *updatefake.Engine { return newOfferEngine(updatefake.Install{Err: errors.New("disk full")}) },
			drive:  checkOnce,
			status: update.StatusFailed, label: "Download Nevermind 2.1.0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMenuUpdates(t, tc.engine())
			if tc.drive != nil {
				tc.drive(t, m)
			}
			assertOneXMenu(t, m, m.waitForStatus(t, tc.status), tc.label, tc.disabled)
		})
	}
}

func TestDownloadingMenuDoesNotShowProgress(t *testing.T) {
	m := newMenuUpdates(t, newOfferEngine(updatefake.Install{Progress: []int64{25, 75}, Total: 100, Block: true}))
	checkOnce(t, m)
	m.waitForChanges(t, 5)
	assertOneXMenu(t, m, m.waitForStatus(t, update.StatusDownloading), "Download Nevermind 2.1.0", false)
}

func TestInstallRowOpensTheUpdatesPageAndRestarts(t *testing.T) {
	engine := newOfferEngine(updatefake.Install{})
	m := newMenuUpdates(t, engine)
	checkOnce(t, m)
	m.waitForStatus(t, update.StatusDownloaded)
	m.click(t, "update")
	if title, shown := m.topPage(); title != "Updates" || !shown {
		t.Fatalf("top page = %q shown %v, want the Updates page on screen", title, shown)
	}
	if engine.Relaunches() != 1 {
		t.Fatalf("relaunches = %d, want 1", engine.Relaunches())
	}
}

func TestDownloadRowOpensTheUpdatesPage(t *testing.T) {
	m := newMenuUpdates(t, newOfferEngine(updatefake.Install{Block: true}))
	checkOnce(t, m)
	m.waitForStatus(t, update.StatusDownloading)
	m.click(t, "update")
	if title, shown := m.topPage(); title != "Updates" || !shown {
		t.Fatalf("top page = %q shown %v, want the Updates page on screen", title, shown)
	}
}

func TestCheckRowOpensTheUpdatesPageAndChecks(t *testing.T) {
	engine := updatefake.NewEngine([]updatefake.Check{{}})
	m := newMenuUpdates(t, engine)
	m.click(t, "check-updates")
	if title, shown := m.topPage(); title != "Updates" || !shown {
		t.Fatalf("top page = %q shown %v, want the Updates page on screen", title, shown)
	}
	eventually(t, "the check to run", func() bool { return engine.Checks() == 1 })
}
