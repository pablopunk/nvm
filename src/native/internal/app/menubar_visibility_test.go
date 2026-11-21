package app

import "testing"

// The row action hides an extension's menu bar items and shows them again, and each choice is refreshed on the palette.
func TestMenuBarVisibilityActionTogglesAnExtensionsItems(t *testing.T) {
	p, _ := wiredApp(t)
	const id = "acme.menubar"
	if !p.visibility.shown(id) {
		t.Fatal("a new extension's menu bar items are hidden; 1.x shows them by default")
	}
	host := &recordingHost{}
	if title := p.menuBarVisibilityAction(id).Title; title != "Hide in Menu Bar" {
		t.Fatalf("action = %q, want Hide in Menu Bar", title)
	}
	p.menuBarVisibilityAction(id).Run(host)
	if p.visibility.shown(id) || host.refresh != 1 {
		t.Fatalf("after hiding: shown %v, refreshes %d", p.visibility.shown(id), host.refresh)
	}
	if title := p.menuBarVisibilityAction(id).Title; title != "Show in Menu Bar" {
		t.Fatalf("hidden action = %q, want Show in Menu Bar", title)
	}
	p.menuBarVisibilityAction(id).Run(host)
	if !p.visibility.shown(id) || host.refresh != 2 {
		t.Fatalf("after showing: shown %v, refreshes %d", p.visibility.shown(id), host.refresh)
	}
}
