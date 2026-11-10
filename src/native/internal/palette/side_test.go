package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// splitRoot is a page with rows of its own whose body is split beside side, as a builder chat is beside its preview.
func splitRoot(main *View, side *Side) *View {
	main.Side = func() *Side { return side }
	return main
}

// newSplitHarness opens the palette on a split page at the split's window size, shown with the keyboard on the page.
func newSplitHarness(t *testing.T, root *View) *harness {
	t.Helper()
	return newSizedHarness(t, root, true, SplitWidth, SplitHeight)
}

// sideChosenTitle is the title of the side's chosen row, or "" without rows.
func sideChosenTitle(side *Side) string {
	if it := side.shell.chosenItem(); it != nil {
		return it.Title
	}
	return ""
}

// TestSideIsDrawnBesideThePageAndItsRowsAreItsOwn checks both pages show their own rows, and the rows of one do not
// become the rows of the other.
func TestSideIsDrawnBesideThePageAndItsRowsAreItsOwn(t *testing.T) {
	var ran []string
	side := NewSide(rowsView("Preview", &ran, "Local time", "Date"))
	h := newSplitHarness(t, splitRoot(rowsView("Chat", &ran, "Main one", "Main two"), side))
	if !h.tt.HasText("Local time") || !h.tt.HasText("Main one") {
		t.Fatal("the split does not show both pages")
	}
	if got := side.shell.Results(); len(got) != 2 || got[0].Title != "Local time" {
		t.Fatalf("side rows = %v, want its own two rows", got)
	}
	if got := h.p.Results(); len(got) != 2 || got[0].Title != "Main one" {
		t.Fatalf("page rows = %v, want the page's own two rows", got)
	}
}

// TestFocusedSideTakesTheKeysAndTyping checks the arrows and typing reach the side only while it has the focus, and
// that the page keeps its own selection and query.
func TestFocusedSideTakesTheKeysAndTyping(t *testing.T) {
	var ran []string
	side := NewSide(rowsView("Preview", &ran, "Local time", "Date", "Dice"))
	h := newSplitHarness(t, splitRoot(rowsView("Chat", &ran, "Main one", "Main two"), side))

	side.Focus()
	h.key(0, ui.KeyDown)
	if got := sideChosenTitle(side); got != "Date" {
		t.Fatalf("with the side focused, ArrowDown chose %q in the side, want Date", got)
	}
	if got := h.chosenTitle(); got != "Main one" {
		t.Fatalf("with the side focused, the page chose %q, want Main one", got)
	}
	h.typeText("da")
	if got := side.shell.Query(); got != "da" {
		t.Fatalf("typing went to the side query %q, want da", got)
	}
	if got := h.p.Query(); got != "" {
		t.Fatalf("typing reached the page query %q while the side had the focus", got)
	}
}

// TestEscapePopsTheSidePageThenGivesTheFocusBack checks Escape is the side's: a pushed page of the side is left first,
// then the focus goes back to the page, which keeps the palette open.
func TestEscapePopsTheSidePageThenGivesTheFocusBack(t *testing.T) {
	var ran []string
	side := NewSide(rowsView("Preview", &ran, "Local time", "Date"))
	h := newSplitHarness(t, splitRoot(rowsView("Chat", &ran, "Main one", "Main two"), side))
	side.Focus()
	side.shell.Push(rowsView("Detail", &ran, "Inside"))
	h.frame()

	h.key(0, ui.KeyEscape)
	if !side.Focused() || len(side.shell.stack) != 1 {
		t.Fatalf("Escape on a pushed side page: focused=%v depth=%d, want focused on the first page", side.Focused(), len(side.shell.stack))
	}
	h.key(0, ui.KeyEscape)
	if side.Focused() {
		t.Fatal("Escape on the side's first page did not give the focus back")
	}
	if !h.win.visible {
		t.Fatal("Escape in the side hid the palette")
	}
	h.typeText("m")
	if got := h.p.Query(); got != "m" {
		t.Fatalf("after the focus came back, typing reached the page query %q, want m", got)
	}
}

// TestSideRefreshShowsTheRowsItHasNow checks a refresh searches the side again: rows that changed after the last search
// show on the next frame.
func TestSideRefreshShowsTheRowsItHasNow(t *testing.T) {
	var ran []string
	titles := []string{"Local time"}
	side := NewSide(&View{Title: "Preview", HideInput: true, Search: func(string) []search.Item {
		var out []search.Item
		for _, title := range titles {
			out = append(out, search.Item{Kind: "Test", Title: title})
		}
		return out
	}})
	h := newSplitHarness(t, splitRoot(rowsView("Chat", &ran, "Main one"), side))
	titles = []string{"Local time", "Date"}
	side.Refresh()
	h.win.drain()
	h.frame()
	if !h.tt.HasText("Date") {
		t.Fatal("the side did not show its new row after Refresh")
	}
}

// TestSetRootReplacesTheSidePage checks SetRoot swaps the side's first page and drops the pages pushed above it.
func TestSetRootReplacesTheSidePage(t *testing.T) {
	var ran []string
	side := NewSide(rowsView("Preview", &ran, "Old row"))
	h := newSplitHarness(t, splitRoot(rowsView("Chat", &ran, "Main one"), side))
	side.shell.Push(rowsView("Detail", &ran, "Inside"))
	side.SetRoot(rowsView("Fresh", &ran, "New row"))
	h.frame()
	if h.tt.HasText("Old row") || !h.tt.HasText("New row") {
		t.Fatal("SetRoot did not replace the side's page")
	}
	if len(side.shell.stack) != 1 {
		t.Fatalf("SetRoot left %d side pages, want the new root alone", len(side.shell.stack))
	}
}

// TestSplitSizesTheWindowAsOneOneXDoes checks the presentations ask for 1.x's window sizes: a page split beside a side
// is the preview size, a side-preview list the side-preview size, and a plain page the default size.
func TestSplitSizesTheWindowAsOneOneXDoes(t *testing.T) {
	var sizes [][2]int
	var ran []string
	side := NewSide(rowsView("Preview", &ran, "Local time"))
	root := splitRoot(rowsView("Chat", &ran, "Main one"), side)
	h := newHarness(t, root, true, func(o *Options) {
		o.Resize = func(w, h int) { sizes = append(sizes, [2]int{w, h}) }
	})
	h.frame()
	if last := sizes[len(sizes)-1]; last != [2]int{SplitWidth, SplitHeight} {
		t.Fatalf("a split page sized the window to %v, want %dx%d", last, SplitWidth, SplitHeight)
	}

	sidePreview := rowsView("Items", &ran, "Item")
	sidePreview.Presentation = SidePreview
	h.p.Push(sidePreview)
	h.frame()
	if last := sizes[len(sizes)-1]; last != [2]int{SidePreviewWidth, SidePreviewHeight} {
		t.Fatalf("a side-preview list sized the window to %v, want %dx%d", last, SidePreviewWidth, SidePreviewHeight)
	}

	h.p.Pop()
	h.frame()
	if last := sizes[len(sizes)-1]; last != [2]int{SplitWidth, SplitHeight} {
		t.Fatalf("the split page sized the window to %v after a pop, want %dx%d", last, SplitWidth, SplitHeight)
	}

	var plainSizes [][2]int
	newHarness(t, rowsView("Plain", &ran, "Row"), true, func(o *Options) {
		o.Resize = func(w, h int) { plainSizes = append(plainSizes, [2]int{w, h}) }
	})
	if last := plainSizes[len(plainSizes)-1]; last != [2]int{DefaultWidth, DefaultHeight} {
		t.Fatalf("the plain page sized the window to %v, want %dx%d", last, DefaultWidth, DefaultHeight)
	}
}
