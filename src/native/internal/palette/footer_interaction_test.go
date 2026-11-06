package palette

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

func footerGrid(title string, count int) *View {
	names := make([]string, count)
	for i := range names {
		names[i] = fmt.Sprintf("Cell %02d", i)
	}
	v := rowsView(title, nil, names...)
	v.Grid = &Grid{Columns: 3, CellHeight: 40, Cell: func(c *ui.Context, item search.Item) { ui.Text(c, item.Title) }}
	v.Primary = func() *design.KeyHint { return &design.KeyHint{Title: "Primary", Key: "↵"} }
	return v
}

func TestGridSelectionClearsFooterAtNormalAndShortHeights(t *testing.T) {
	for _, height := range []int{DefaultHeight, 250} {
		t.Run(fmt.Sprintf("height=%d", height), func(t *testing.T) {
			h := newSizedHarness(t, footerGrid("Grid", 17), true, DefaultWidth, height)
			for range 6 { // 17 items: the final row contains only two cells.
				h.key(0, ui.KeyDown)
			}
			cell, ok := h.tt.Find("Cell 16")
			footer, footerOK := h.tt.Find("Primary")
			if !ok || !footerOK || cell.Y+cell.H > footer.Y {
				t.Fatalf("last grid cell does not clear footer: cell=%v found=%t footer=%v found=%t", cell, ok, footer, footerOK)
			}
		})
	}
}

func TestWheelScrollPersistsUntilKeyboardSelectionChanges(t *testing.T) {
	h := newSizedHarness(t, footerGrid("Grid", 30), true, DefaultWidth, 250)
	initial, initialOK := h.tt.Find("Cell 09")
	if !initialOK {
		t.Fatal("wheel anchor missing before scrolling")
	}
	h.tt.Wheel(350, 180, 0, 100)
	h.tt.Advance(500 * time.Millisecond)
	h.frame()
	before, beforeOK := h.tt.Find("Cell 09")
	if !beforeOK || before.Y == initial.Y {
		t.Fatal("wheel did not move visible content")
	}
	h.frame()
	after, afterOK := h.tt.Find("Cell 09")
	if !afterOK || before.Y != after.Y {
		t.Fatalf("idle frame snapped wheel scroll back: before=%v after=%v", before, after)
	}
	h.key(0, ui.KeyDown)
	cell, ok := h.tt.Find("Cell 03")
	footer, footerOK := h.tt.Find("Primary")
	if !ok || !footerOK || cell.Y+cell.H > footer.Y {
		t.Fatalf("keyboard selection did not clear footer after wheel scroll: cell=%v found=%t footer=%v found=%t", cell, ok, footer, footerOK)
	}
}

func TestFooterRevealRefreshesAfterPopulationAndResize(t *testing.T) {
	pending := true
	v := rowsView("List", nil)
	v.Primary = func() *design.KeyHint { return &design.KeyHint{Title: "Primary", Key: "↵"} }
	v.Search = func(string) []search.Item {
		if pending {
			return nil
		}
		items := make([]search.Item, 18)
		for i := range items {
			items[i] = search.Item{Kind: "Test", Title: fmt.Sprintf("Result %02d", i), Actions: []search.Action{{Title: "Primary", Run: func(search.Host) {}}}}
		}
		return items
	}
	h := newSizedHarness(t, v, true, DefaultWidth, 470)
	pending = false
	h.p.Refresh()
	h.frame()
	h.p.choose(17)
	h.frame()
	selected, ok := h.tt.Find("Result 17")
	footer, footerOK := h.tt.Find("Primary")
	if !ok || !footerOK || selected.Y+selected.H > footer.Y {
		t.Fatalf("populated selection overlaps footer: row=%v found=%t footer=%v found=%t", selected, ok, footer, footerOK)
	}
	h.tt.SetSize(DefaultWidth, 250)
	h.frame()
	selected, ok = h.tt.Find("Result 17")
	footer, footerOK = h.tt.Find("Primary")
	if !ok || !footerOK || selected.Y+selected.H > footer.Y {
		t.Fatalf("selection geometry stale after resize: row=%v found=%t footer=%v found=%t", selected, ok, footer, footerOK)
	}
}

func TestGridSelectionRevealedAfterResize(t *testing.T) {
	h := newSizedHarness(t, footerGrid("Grid", 60), true, DefaultWidth, 470)
	h.p.choose(59)
	h.frame()
	h.tt.SetSize(DefaultWidth, 250)
	h.frame()
	cell, ok := h.tt.Find("Cell 59")
	footer, footerOK := h.tt.Find("Primary")
	if !ok || !footerOK || cell.Y+cell.H > footer.Y {
		t.Fatalf("grid selection not revealed after resize: cell=%v found=%t footer=%v found=%t", cell, ok, footer, footerOK)
	}
}
