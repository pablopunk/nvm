package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Without the extension host there is no view to show, so a click on a popover item opens nothing and keeps no page.
func TestPopoverClickWithoutTheExtensionHostOpensNothing(t *testing.T) {
	p, _ := wiredApp(t)
	p.exts.mgr.Store(nil)
	window := &stubWindow{}
	p.Attach(window)
	host := popoverHost{p: p}
	popover := api.Popover{View: api.ListView("Timer", api.Item{ID: "row", Title: "Remaining"})}
	host.Toggle("acme.menubar", "main", popover)
	if window.shown || p.popover.view != nil {
		t.Fatalf("shown %v, page %v; want no popover without the host", window.shown, p.popover.view)
	}
	host.Refresh("acme.menubar", "main", popover)
	if p.popover.view != nil {
		t.Fatal("a refresh opened a popover that was never shown")
	}
}
