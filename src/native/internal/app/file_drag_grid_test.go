package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TestPressDraggingAGridCellDragsItsFileAndChoosesNothing covers a grid cell: a press that moves drags the cell's file
// out and leaves the cell unchosen, and a click on the cell still chooses it.
func TestPressDraggingAGridCellDragsItsFileAndChoosesNothing(t *testing.T) {
	const shot = "/Users/alice/Pictures/shot.png"
	set := fake.NewSet()
	app := &App{ports: set.Ports()}
	ran := &[]string{}
	cells := []search.Item{
		{Title: "a.txt"},
		{Title: "shot.png", DragPaths: pathsOf(shot), Actions: []search.Action{{Title: "Open", Run: func(search.Host) { *ran = append(*ran, "shot.png") }}}},
	}
	root := &palette.View{
		Title:  "Grid",
		Search: func(string) []search.Item { return cells },
		Grid: &palette.Grid{Columns: 2, CellHeight: 96, Cell: func(c *ui.Context, it search.Item) {
			ui.Text(c, it.Title)
		}},
	}
	pal := palette.New(palette.Options{Root: root, Clock: fake.NewClock(), Synchronous: true, DragRow: app.dragRow})
	pal.Attach(&dragWindow{})
	pal.Show()
	tt := ui.NewTester(pal.View, 720, 470)
	tt.Frame()

	x, y := centerOf(t, tt, "shot.png")
	tt.Press(x, y)
	tt.Move(x+30, y+2)
	tt.Frame()
	tt.Release(x+30, y+2)
	tt.Frame()

	calls := set.FileDrag.Calls()
	if len(calls) != 1 || len(calls[0].Paths) != 1 || calls[0].Paths[0] != shot {
		t.Fatalf("drag calls = %v, want one drag of %s", calls, shot)
	}
	if got := pal.Chosen(); got != 0 {
		t.Fatalf("a drag chose cell %d, want the first kept", got)
	}
	if len(*ran) != 0 {
		t.Fatalf("a drag ran the cell: %v", *ran)
	}
	if err := tt.Click("shot.png"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := pal.Chosen(); got != 1 {
		t.Fatalf("a click chose cell %d, want the clicked cell 1", got)
	}
}

// centerOf returns the middle of the element showing title, failing the test when none is on screen.
func centerOf(t *testing.T, tt *ui.Tester, title string) (float32, float32) {
	t.Helper()
	rect, ok := tt.Find(title)
	if !ok {
		t.Fatalf("%q is not on screen", title)
	}
	return rect.X + rect.W/2, rect.Y + rect.H/2
}

// TestAGridCellChoosesOnPointerMovementAndRunsOnClick checks 1.x's grid: the pointer moving onto a cell chooses it, and a
// click runs it.
func TestAGridCellChoosesOnPointerMovementAndRunsOnClick(t *testing.T) {
	set := fake.NewSet()
	app := &App{ports: set.Ports()}
	ran := &[]string{}
	cells := []search.Item{
		{Title: "a.txt", Actions: []search.Action{{Title: "Open", Run: func(search.Host) { *ran = append(*ran, "a.txt") }}}},
		{Title: "b.txt", Actions: []search.Action{{Title: "Open", Run: func(search.Host) { *ran = append(*ran, "b.txt") }}}},
	}
	root := &palette.View{
		Title:  "Grid",
		Search: func(string) []search.Item { return cells },
		Grid: &palette.Grid{Columns: 2, CellHeight: 96, Cell: func(c *ui.Context, it search.Item) {
			ui.Text(c, it.Title)
		}},
	}
	pal := palette.New(palette.Options{Root: root, Clock: fake.NewClock(), Synchronous: true, DragRow: app.dragRow})
	pal.Attach(&dragWindow{})
	pal.Show()
	tt := ui.NewTester(pal.View, 720, 470)
	tt.Frame()

	x, y := centerOf(t, tt, "b.txt")
	tt.Move(x, y)
	tt.Frame()
	tt.Move(x+4, y)
	tt.Frame()
	if got := pal.Chosen(); got != 1 {
		t.Fatalf("pointer moved onto cell 1: chosen = %d, want 1", got)
	}
	if len(*ran) != 0 {
		t.Fatalf("pointer movement ran a cell: %v", *ran)
	}
	if err := tt.Click("b.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(*ran) != 1 || (*ran)[0] != "b.txt" {
		t.Fatalf("a click did not run the cell: %v", *ran)
	}
}
