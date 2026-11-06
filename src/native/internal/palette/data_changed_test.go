package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestNotifyTopDataChangedTargetsVisibleExtension(t *testing.T) {
	root := &View{Title: "Root"}
	h := newHarness(t, root, true)
	calls := 0
	page := &View{Title: "Menu Items", ExtensionID: "nevermind.menus", OnDataChanged: func() { calls++ }, Search: func(string) []search.Item {
		return []search.Item{{ID: "open", Title: "Open", Actions: []search.Action{{Title: "Open", Run: func(search.Host) {}}}}}
	}}
	h.p.Push(page)
	h.p.SetQuery("Open")
	h.key(ui.Cmd, ui.KeyK)
	if !h.p.actionsOpen {
		t.Fatal("action panel did not open")
	}
	h.p.NotifyTopDataChanged("other.extension")
	h.p.NotifyTopDataChanged("nevermind.menus")
	h.win.drain()
	if calls != 1 || h.p.cur() != page || h.p.Query() != "Open" || len(h.p.stack) != 2 || !h.p.actionsOpen {
		t.Fatalf("notification changed navigation: calls=%d query=%q depth=%d panel=%t", calls, h.p.Query(), len(h.p.stack), h.p.actionsOpen)
	}
	h.p.NotifyTopDataChanged("nevermind.menus")
	h.p.Push(&View{Title: "Covered"})
	h.win.drain()
	if calls != 1 {
		t.Fatal("queued notification refreshed a covered page")
	}
	h.p.Pop()
	h.p.NotifyTopDataChanged("nevermind.menus")
	h.p.Hide()
	h.win.drain()
	if calls != 1 {
		t.Fatal("queued notification refreshed a hidden page")
	}
	New(Options{}).NotifyTopDataChanged("nevermind.menus")
}
