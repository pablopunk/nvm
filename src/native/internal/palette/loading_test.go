package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// loadingView is a list or a three-column grid of rows. Its second batch is in flight while *pending holds, and its
// rows are *titles, so a test can deliver the batch by changing them.
func loadingView(grid bool, pending *bool, titles *[]string) *View {
	v := &View{
		Title: "Loading",
		Search: func(string) []search.Item {
			out := make([]search.Item, 0, len(*titles))
			for _, name := range *titles {
				out = append(out, search.Item{ID: name, Kind: "Test", Title: name})
			}
			return out
		},
		Pending: func() bool { return *pending },
	}
	if grid {
		v.Grid = &Grid{Columns: 3, CellHeight: 40, Cell: func(*ui.Context, search.Item) {}}
	}
	return v
}

// TestPendingBatchShowsAPlaceholderAfterTheRowsAndTakesNoChoice checks a list with its second batch in flight: the
// rows stay the rows, the placeholder is not a row, and Down from the last row does not move onto it.
func TestPendingBatchShowsAPlaceholderAfterTheRowsAndTakesNoChoice(t *testing.T) {
	pending := true
	titles := []string{"Cat", "Tiger"}
	h := newHarness(t, loadingView(false, &pending, &titles), true)
	h.frame()
	if !h.p.loadingMore() {
		t.Fatal("the list does not show the batch as loading")
	}
	if got := h.titles(); len(got) != 2 {
		t.Fatalf("rows = %v, want the two results and no placeholder row", got)
	}
	h.key(0, ui.KeyDown)
	h.key(0, ui.KeyDown)
	if got := h.chosenTitle(); got != "Tiger" {
		t.Fatalf("Down past the last result chose %q, want Tiger: the placeholder took the choice", got)
	}
}

// TestPendingBatchPlaceholdersFillTheGridSlotsAfterTheResults checks a grid with its batch in flight: the placeholders
// follow the last result, the choice stops on the last result, and Right does not move onto a placeholder cell.
func TestPendingBatchPlaceholdersFillTheGridSlotsAfterTheResults(t *testing.T) {
	pending := true
	titles := gridTitles
	h := newHarness(t, loadingView(true, &pending, &titles), true)
	h.frame()
	if got := len(h.titles()); got != len(gridTitles) {
		t.Fatalf("rows = %d, want the %d results only", got, len(gridTitles))
	}
	for range 3 {
		h.key(0, ui.KeyDown)
	}
	h.key(0, ui.KeyRight)
	h.key(0, ui.KeyRight)
	if got := h.chosenTitle(); got != "G" {
		t.Fatalf("arrows past the last result chose %q, want G", got)
	}
}

// TestBatchArrivingReplacesThePlaceholderAndKeepsTheRows checks the batch landing: the rows it adds show, the
// placeholder goes, and the rows before it keep their places.
func TestBatchArrivingReplacesThePlaceholderAndKeepsTheRows(t *testing.T) {
	pending := true
	titles := []string{"Cat"}
	h := newHarness(t, loadingView(false, &pending, &titles), true)
	h.frame()

	pending = false
	titles = append(titles, "Tiger")
	h.p.Refresh()
	h.frame()
	if h.p.loadingMore() {
		t.Fatal("the placeholder stayed after the batch arrived")
	}
	if got := h.titles(); len(got) != 2 || got[0] != "Cat" || got[1] != "Tiger" {
		t.Fatalf("rows = %v, want Cat then Tiger in their places", got)
	}
}

// TestPendingBatchWithNoResultsShowsThePlaceholderWithoutAChoice checks a query with no local rows and a batch in
// flight: there is nothing to choose, and the placeholder does not make one.
func TestPendingBatchWithNoResultsShowsThePlaceholderWithoutAChoice(t *testing.T) {
	pending := true
	titles := []string{}
	h := newHarness(t, loadingView(true, &pending, &titles), true)
	h.frame()
	if !h.p.loadingMore() || len(h.titles()) != 0 {
		t.Fatalf("loading = %v, rows = %v, want the placeholder and no rows", h.p.loadingMore(), h.titles())
	}
	h.key(0, ui.KeyDown)
	if h.chosenTitle() != "" {
		t.Fatalf("the choice = %q, want none", h.chosenTitle())
	}
}

// TestTypingWhileABatchIsPendingStillChangesTheQuery checks the placeholder never blocks the field.
func TestTypingWhileABatchIsPendingStillChangesTheQuery(t *testing.T) {
	pending := true
	titles := []string{"Cat"}
	h := newHarness(t, loadingView(false, &pending, &titles), true)
	h.frame()
	h.typeText("ca")
	if h.p.Query() != "ca" {
		t.Fatalf("query = %q, want ca while the batch is pending", h.p.Query())
	}
}

func TestRefreshKeepsSelectedIdentityAcrossReordering(t *testing.T) {
	for _, grid := range []bool{false, true} {
		pending := true
		titles := []string{"Cat", "Tiger"}
		h := newHarness(t, loadingView(grid, &pending, &titles), true)
		h.p.Choose(1)
		h.frame()
		titles = []string{"Tiger", "Cat", "Lion"}
		h.p.Refresh()
		h.frame()
		if h.chosenTitle() != "Tiger" {
			t.Fatal("refresh changed the selected item")
		}
		pending = false
		titles = nil
		h.p.Refresh()
		h.frame()
		if h.p.loadingMore() || h.chosenTitle() != "" {
			t.Fatal("empty completion retained loading or a choice")
		}
	}
}

func TestColdLoadingReducedMotionAndSettling(t *testing.T) {
	for _, grid := range []bool{false, true} {
		pending := true
		titles := []string{}
		h := newHarness(t, loadingView(grid, &pending, &titles), true)
		h.tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		h.frame()
		if h.tt.FramePending() {
			t.Fatal("reduced-motion loading kept requesting frames")
		}
		pending = false
		h.p.Refresh()
		h.frame()
		if h.p.loadingMore() || h.tt.FramePending() {
			t.Fatal("settled loading kept requesting frames")
		}
	}
}
