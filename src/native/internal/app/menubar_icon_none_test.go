package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/menubar"
)

// TestNoneIconStyleRemovesTheItemAndAnotherStyleBringsItBack checks the menuBarIconStyle choice None: the item leaves the
// bar at once, and choosing a style again makes a new one.
func TestNoneIconStyleRemovesTheItemAndAnotherStyleBringsItBack(t *testing.T) {
	p, set := wiredApp(t)
	p.setUpMenubar()
	first := set.Tray.Trays()[0]

	p.applyMenuBarIconStyle(menubar.IconStyleNone)
	if !first.Closed() || p.menubar != nil {
		t.Fatalf("closed = %v, item = %v; want the item removed", first.Closed(), p.menubar)
	}

	p.applyMenuBarIconStyle("bolt-outline")
	trays := set.Tray.Trays()
	if p.menubar == nil || len(trays) != 2 || trays[1].Closed() {
		t.Fatalf("item = %v, trays = %d; want a new open item", p.menubar, len(trays))
	}
}

func TestNoneIconStyleAtStartupCreatesNoItem(t *testing.T) {
	p, set := wiredApp(t)
	p.applyMenuBarIconStyle(menubar.IconStyleNone)
	if p.menubar != nil || len(set.Tray.Trays()) != 0 {
		t.Fatalf("item = %v, trays = %d; want none", p.menubar, len(set.Tray.Trays()))
	}
}
