package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/appcommands"
	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
	"github.com/pablopunk/nvm/native/internal/store"
)

// quitCalls counts the quit requests the system port recorded.
func quitCalls(t *testing.T, p *App) int {
	t.Helper()
	count := 0
	for _, call := range fakeSystem(t, p).Calls() {
		if call.Op == fake.OpSystemQuit {
			count++
		}
	}
	return count
}

// TestQuitNevermindReachesTheSystemQuit runs the palette's Quit Nevermind op and checks the quit reaches the system
// port, which is the call MyGo's quit sits behind.
func TestQuitNevermindReachesTheSystemQuit(t *testing.T) {
	p := goldenPalette(t)
	done := false
	p.runSystemOp(appcommands.QuitOp, func(failure string) { done = failure == "" })
	if got := quitCalls(t, p); got != 1 {
		t.Fatalf("quit calls = %d, want 1", got)
	}
	if !done {
		t.Fatal("Quit Nevermind reported a failure")
	}
}

func TestRestartNevermindRequestsRelaunch(t *testing.T) {
	p := goldenPalette(t)
	done := false
	p.runSystemOp(appcommands.RestartOp, func(failure string) { done = failure == "" })
	calls := fakeSystem(t, p).Calls()
	if !done || len(calls) != 1 || calls[0].Op != fake.OpSystemRelaunch {
		t.Fatalf("relaunch calls = %+v, done = %v", calls, done)
	}
}

// TestMenubarQuitReachesTheSystemQuit checks the menu bar's Quit row takes the same path as Quit Nevermind.
func TestMenubarQuitReachesTheSystemQuit(t *testing.T) {
	p := goldenPalette(t)
	p.menubarActions().Quit()
	if got := quitCalls(t, p); got != 1 {
		t.Fatalf("quit calls = %d, want 1", got)
	}
}

// TestPanelCloseGivesWayToAQuit checks the palette's window does not refuse its close while the app quits. MyGo
// cancels a quit that any window refuses, which is how the quit used to do nothing.
func TestPanelCloseGivesWayToAQuit(t *testing.T) {
	p := &App{quitting: true}
	e := &mygo.CloseEvent{}
	p.hidePanelUnlessQuitting(e)
	if e.DefaultPrevented() {
		t.Fatal("the panel refused its close during a quit, so MyGo cancels the quit")
	}
}

// TestQuitRemovesTheMenubarItem checks the menu bar item is closed when the quit reaches its window-close step.
func TestQuitRemovesTheMenubarItem(t *testing.T) {
	host := fake.NewTrayHost()
	item, err := menubar.New(menubar.Options{Host: host, OnUI: func(fn func()) { fn() }})
	if err != nil {
		t.Fatalf("menubar.New: %v", err)
	}
	p := &App{menubar: item}
	p.closeMenubarItems(&mygo.QuitEvent{})
	trays := host.Trays()
	if len(trays) != 1 || !trays[0].Closed() {
		t.Fatalf("menu bar item closed = %v, want the item removed", trays)
	}
}

// TestQuitReleasesTheStoreLease checks that closing the store on quit releases its lease: a second instance can take
// the directory only after the first has closed it.
func TestQuitReleasesTheStoreLease(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	dir := testpath.Abs("quit", "store")
	lease, err := store.AcquireLease(files, clock, dir, store.WithHeartbeat(0, 30*time.Second))
	if err != nil {
		t.Fatalf("AcquireLease: %v", err)
	}
	st, err := store.Open(files, clock, dir, store.WithLease(lease))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if _, err := store.AcquireLease(files, clock, dir, store.WithHeartbeat(0, 30*time.Second)); err == nil {
		t.Fatal("a second instance took the lease while the first still held the store")
	}
	closeStoreOnQuit(st)()
	if _, err := store.AcquireLease(files, clock, dir, store.WithHeartbeat(0, 30*time.Second)); err != nil {
		t.Fatalf("the lease after the quit: %v", err)
	}
}
