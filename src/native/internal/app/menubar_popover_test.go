package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// popoverTestView is the popover content the tests open: one list with one row.
func popoverTestView(width, height int) api.Popover {
	return api.Popover{View: api.ListView("Timer", api.Item{ID: "row", Title: "Remaining"}), Width: width, Height: height}
}

// trayItem is a fake menu bar item of the test app.
func trayItem(t *testing.T, set *fake.Set) *fake.Tray {
	t.Helper()
	tray, err := set.Tray.NewTray(platform.TrayOptions{ID: "ext:acme.menubar:main"})
	if err != nil {
		t.Fatalf("NewTray: %v", err)
	}
	return tray.(*fake.Tray)
}

// Without the extension host there is no view to show, so a click opens no popover window.
func TestPopoverClickWithoutTheExtensionHostOpensNothing(t *testing.T) {
	p, set := wiredApp(t)
	p.exts.mgr.Store(nil)
	tray := trayItem(t, set)
	popoverHost{p: p}.Toggle("acme.menubar", "main", tray, popoverTestView(360, 420))
	if got := len(tray.Popovers()); got != 0 {
		t.Fatalf("popovers opened = %d, want none without the host", got)
	}
}

// A click opens the popover window under its tray item at the declared size, the second click closes it, and a third
// shows the same window again.
func TestPopoverOpensUnderItsTrayItemAtItsSize(t *testing.T) {
	p, set := wiredApp(t)
	tray := trayItem(t, set)
	host := popoverHost{p: p}
	host.Toggle("acme.menubar", "main", tray, popoverTestView(400, 300))
	popovers := tray.Popovers()
	if len(popovers) != 1 {
		t.Fatalf("popovers = %d, want one window under the item", len(popovers))
	}
	if got := popovers[0].Options(); got != (platform.PopoverOptions{Width: 400, Height: 300}) {
		t.Fatalf("popover size = %+v, want the declared 400x300", got)
	}
	if !popovers[0].IsVisible() {
		t.Fatal("the first click did not show the popover")
	}
	host.Toggle("acme.menubar", "main", tray, popoverTestView(400, 300))
	if popovers[0].IsVisible() {
		t.Fatal("the second click did not close the popover")
	}
	host.Toggle("acme.menubar", "main", tray, popoverTestView(400, 300))
	if !popovers[0].IsVisible() || len(tray.Popovers()) != 1 {
		t.Fatalf("a third click showed %v with %d windows, want the same window again", popovers[0].IsVisible(), len(tray.Popovers()))
	}
}

// A render while the popover is shown refreshes its page and keeps it shown.
func TestOpenPopoverFollowsItsRenders(t *testing.T) {
	p, set := wiredApp(t)
	tray := trayItem(t, set)
	host := popoverHost{p: p}
	host.Toggle("acme.menubar", "main", tray, popoverTestView(360, 420))
	host.Refresh("acme.menubar", "main", tray, popoverTestView(360, 420))
	if !tray.Popovers()[0].IsVisible() {
		t.Fatal("a refresh closed the open popover")
	}
}
