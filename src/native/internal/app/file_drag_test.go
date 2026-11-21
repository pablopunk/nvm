package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// dragWindow is the palette's window in these tests. The palette only asks it to update, so updates run at once.
type dragWindow struct{ visible bool }

func (w *dragWindow) Update(fn func())     { fn() }
func (w *dragWindow) Show()                { w.visible = true }
func (w *dragWindow) Hide()                { w.visible = false }
func (w *dragWindow) IsVisible() bool      { return w.visible }
func (w *dragWindow) IsFocused() bool      { return w.visible }
func (w *dragWindow) Focus()               {}
func (w *dragWindow) Size() (int, int)     { return 720, 470 }
func (w *dragWindow) SetPosition(x, y int) {}

// dragHarness is a palette whose rows drag through the real row wrapper and a fake drag port.
type dragHarness struct {
	t    *testing.T
	pal  *palette.Palette
	drag *fake.FileDrag
	tt   *ui.Tester
	ran  *[]string
}

// newDragHarness opens a palette on rows. Each row runs an action that records its title.
func newDragHarness(t *testing.T, rows ...search.Item) *dragHarness {
	t.Helper()
	set := fake.NewSet()
	app := &App{ports: set.Ports()}
	ran := &[]string{}
	for i := range rows {
		title := rows[i].Title
		rows[i].Actions = []search.Action{{Title: "Open", Run: func(search.Host) { *ran = append(*ran, title) }}}
	}
	root := &palette.View{Title: "Root", Search: func(string) []search.Item { return rows }}
	pal := palette.New(palette.Options{
		Root:        root,
		Clock:       fake.NewClock(),
		Synchronous: true,
		DragRow:     app.dragRow,
	})
	pal.Attach(&dragWindow{})
	pal.Show()
	tt := ui.NewTester(pal.View, 720, 470)
	tt.Frame()
	return &dragHarness{t: t, pal: pal, drag: set.FileDrag, tt: tt, ran: ran}
}

// center returns the middle of the row titled title, failing the test when no such row is on screen.
func (h *dragHarness) center(title string) (float32, float32) {
	h.t.Helper()
	rect, ok := h.tt.Find(title)
	if !ok {
		h.t.Fatalf("row %q is not on screen", title)
	}
	return rect.X + rect.W/2, rect.Y + rect.H/2
}

// pressDrag presses the row and moves the pointer well past the drag threshold, then lets go.
func (h *dragHarness) pressDrag(title string) {
	h.t.Helper()
	x, y := h.center(title)
	h.tt.Press(x, y)
	h.tt.Move(x+30, y+2)
	h.tt.Frame()
	h.tt.Release(x+30, y+2)
	h.tt.Frame()
}

// chosenTitle returns the title of the row the palette has chosen.
func (h *dragHarness) chosenTitle() string {
	rows := h.pal.Results()
	if i := h.pal.Chosen(); i >= 0 && i < len(rows) {
		return rows[i].Title
	}
	return ""
}

// assertDragged checks that exactly one drag started, carrying paths and an image of the drag size.
func (h *dragHarness) assertDragged(paths ...string) image.Image {
	h.t.Helper()
	calls := h.drag.Calls()
	if len(calls) != 1 {
		h.t.Fatalf("drag port calls = %d, want 1", len(calls))
	}
	if got := calls[0].Paths; len(got) != len(paths) || (len(got) > 0 && got[0] != paths[0]) {
		h.t.Fatalf("drag paths = %v, want %v", got, paths)
	}
	if calls[0].Preview == nil || calls[0].Preview.Bounds().Dx() == 0 {
		h.t.Fatal("drag carried no image")
	}
	return calls[0].Preview
}

// TestPressDraggingAnAppRowDragsItsBundleAndRunsNothing covers the owner's report: an application row drags its .app
// bundle, and the press that drags is neither a choice nor a run.
func TestPressDraggingAnAppRowDragsItsBundleAndRunsNothing(t *testing.T) {
	const bundle = "/Applications/Safari.app"
	h := newDragHarness(t, search.Item{Title: "Safari", DragPaths: pathsOf(bundle)})
	h.pressDrag("Safari")
	h.assertDragged(bundle)
	if len(*h.ran) != 0 {
		t.Fatalf("a drag ran the row: %v", *h.ran)
	}
}

// TestPressDraggingAFileRowDragsTheFile covers a file or folder row.
func TestPressDraggingAFileRowDragsTheFile(t *testing.T) {
	const file = "/Users/alice/Notes/plan.md"
	h := newDragHarness(t, search.Item{Title: "plan.md", DragPaths: pathsOf(file)})
	h.pressDrag("plan.md")
	h.assertDragged(file)
	if len(*h.ran) != 0 {
		t.Fatalf("a drag ran the row: %v", *h.ran)
	}
}

// TestPressDraggingAnImageRowDragsTheImageAsItsPicture covers an image result: the drag shows the image file itself.
func TestPressDraggingAnImageRowDragsTheImageAsItsPicture(t *testing.T) {
	path := writeDragPNG(t, 320, 240, color.NRGBA{R: 200, G: 40, B: 40, A: 255})
	h := newDragHarness(t, search.Item{Title: "shot.png", DragPaths: pathsOf(path)})
	h.pressDrag("shot.png")
	picture := h.assertDragged(path)
	if got := picture.Bounds(); got.Dx() != platform.FileDragSize || got.Dy() != platform.FileDragSize {
		t.Fatalf("image bounds = %v, want a %d square", got, platform.FileDragSize)
	}
	if r, _, _, _ := picture.At(platform.FileDragSize/2, platform.FileDragSize/2).RGBA(); r>>8 != 200 {
		t.Fatalf("drag picture is not the image file: center %v", picture.At(platform.FileDragSize/2, platform.FileDragSize/2))
	}
}

// TestPressDraggingARowChoosesNothing checks that a drag leaves the choice where it was: the first row stays chosen
// while a later file row is dragged out.
func TestPressDraggingARowChoosesNothing(t *testing.T) {
	h := newDragHarness(t,
		search.Item{Title: "Safari"},
		search.Item{Title: "plan.md", DragPaths: pathsOf("/Users/alice/Notes/plan.md")},
	)
	h.pressDrag("plan.md")
	if got := h.chosenTitle(); got != "Safari" {
		t.Fatalf("chosen = %q after a drag, want the first row kept", got)
	}
	if len(*h.ran) != 0 {
		t.Fatalf("a drag ran a row: %v", *h.ran)
	}
}

// TestAClickRunsARowWithADragPath checks the click path, as 1.x's cmdk item does: a single click chooses the row and
// runs it, and starts no drag.
func TestAClickRunsARowWithADragPath(t *testing.T) {
	h := newDragHarness(t,
		search.Item{Title: "Alpha"},
		search.Item{Title: "Safari", DragPaths: pathsOf("/Applications/Safari.app")},
	)
	if err := h.tt.Click("Safari"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if len(h.drag.Calls()) != 0 {
		t.Fatalf("a click started a drag: %v", h.drag.Calls())
	}
	if len(*h.ran) != 1 || (*h.ran)[0] != "Safari" || h.chosenTitle() != "Safari" {
		t.Fatalf("one click ran %v and chose %q, want Safari run and chosen", *h.ran, h.chosenTitle())
	}
}

// TestEachClickOfADoubleClickRunsTheRow checks 1.x parity: cmdk runs an item on every click, so a double click runs
// the row twice, and the list's double-click submit adds no third run.
func TestEachClickOfADoubleClickRunsTheRow(t *testing.T) {
	h := newDragHarness(t, search.Item{Title: "Calc"})
	if err := h.tt.Click("Calc"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if err := h.tt.Click("Calc"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	if len(*h.ran) != 2 {
		t.Fatalf("double click ran the row %d times, want 2 as in 1.x: %v", len(*h.ran), *h.ran)
	}
}

// TestPointerMovementChoosesTheRowUnderIt checks 1.x's pointer selection: moving the pointer onto a row chooses it,
// and runs nothing.
func TestPointerMovementChoosesTheRowUnderIt(t *testing.T) {
	h := newDragHarness(t, search.Item{Title: "Alpha"}, search.Item{Title: "Beta"})
	x, y := h.center("Beta")
	h.tt.Move(x, y)
	h.tt.Frame()
	h.tt.Move(x+4, y)
	h.tt.Frame()
	if got := h.chosenTitle(); got != "Beta" {
		t.Fatalf("chosen = %q after the pointer moved onto Beta, want Beta", got)
	}
	if len(*h.ran) != 0 {
		t.Fatalf("pointer movement ran a row: %v", *h.ran)
	}
}

// TestAStillPointerDoesNotUndoKeyboardMovement checks that a choice moved without the pointer moving is kept: a list
// that scrolls under a still pointer must not choose the row that comes under it.
func TestAStillPointerDoesNotUndoKeyboardMovement(t *testing.T) {
	h := newDragHarness(t, search.Item{Title: "Alpha"}, search.Item{Title: "Beta"})
	x, y := h.center("Alpha")
	h.tt.Move(x, y)
	h.tt.Frame()
	h.tt.Move(x, y)
	h.tt.Frame()
	h.pal.Choose(1)
	h.tt.Frame()
	h.tt.Frame()
	if got := h.chosenTitle(); got != "Beta" {
		t.Fatalf("chosen = %q, want the keyboard's Beta kept while the pointer stands still", got)
	}
}

// TestAPressThatDragsChoosesNothingEvenOverOtherRows checks a drag over other rows: the pointer's moves while the
// press drags choose no row, so the choice stays where it was.
func TestAPressThatDragsChoosesNothingEvenOverOtherRows(t *testing.T) {
	h := newDragHarness(t,
		search.Item{Title: "Alpha"},
		search.Item{Title: "Beta", DragPaths: pathsOf("/Users/alice/Notes/beta.md")},
		search.Item{Title: "Gamma"},
	)
	x, y := h.center("Beta")
	h.tt.Press(x, y)
	h.tt.Move(x+30, y+2)
	h.tt.Frame()
	gx, gy := h.center("Gamma")
	h.tt.Move(gx, gy)
	h.tt.Frame()
	h.tt.Release(gx, gy)
	h.tt.Frame()
	if got := h.chosenTitle(); got != "Alpha" {
		t.Fatalf("chosen = %q after a drag over other rows, want the first kept", got)
	}
	if len(*h.ran) != 0 {
		t.Fatalf("a drag ran a row: %v", *h.ran)
	}
}

// TestARowWithoutAPathNeverDrags checks that a press-drag on a plain row starts no drag and still runs nothing.
func TestARowWithoutAPathNeverDrags(t *testing.T) {
	h := newDragHarness(t, search.Item{Title: "Calculator"})
	h.pressDrag("Calculator")
	if len(h.drag.Calls()) != 0 {
		t.Fatalf("a row with no path dragged: %v", h.drag.Calls())
	}
}

// writeDragPNG stores a w×h image filled with c in a temporary folder and returns its path.
func writeDragPNG(t *testing.T, w, h int, c color.Color) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	path := filepath.Join(t.TempDir(), "shot.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	return path
}
