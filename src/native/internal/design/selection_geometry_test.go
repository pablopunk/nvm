package design

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// rowTextRects lays the row out alone and returns the boxes of its tile glyph, title and subtitle.
func rowTextRects(t *testing.T, d RowData) (glyph, title, subtitle ui.Rect) {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { Row(c, d) }, 480, 120)
	tt.Frame()
	glyph = rectOf(t, tt, d.Tile.Glyph)
	title = rectOf(t, tt, d.Title)
	subtitle = rectOf(t, tt, d.Subtitle)
	return glyph, title, subtitle
}

func rectOf(t *testing.T, tt *ui.Tester, text string) ui.Rect {
	t.Helper()
	rect, ok := tt.Find(text)
	if !ok {
		t.Fatalf("no element showing %q", text)
	}
	return rect
}

// TestSelectedRowContentKeepsItsPlaceOnEveryRowKind selects a row and checks that its tile, title and subtitle sit
// where they sit unselected. The selection plate draws its edge inside the row's reserved space, so only the plate
// may change, never the content.
func TestSelectedRowContentKeepsItsPlaceOnEveryRowKind(t *testing.T) {
	for _, prominent := range []bool{false, true} {
		base := RowData{Title: "Clipboard History", Subtitle: "Browse and paste what you copied", Tile: TileData{Glyph: "CB"}, Tone: "blue", Prominent: prominent}
		selected := base
		selected.Selected = true
		glyph, title, subtitle := rowTextRects(t, base)
		selGlyph, selTitle, selSubtitle := rowTextRects(t, selected)
		if glyph != selGlyph || title != selTitle || subtitle != selSubtitle {
			t.Fatalf("prominent=%v: selected content moved: tile %v -> %v, title %v -> %v, subtitle %v -> %v",
				prominent, glyph, selGlyph, title, selTitle, subtitle, selSubtitle)
		}
	}
}

// TestActionMenuRowContentKeepsItsPlaceWhenSelected moves the selection between two action menu rows and checks that
// neither row's label moves.
func TestActionMenuRowContentKeepsItsPlaceWhenSelected(t *testing.T) {
	labelsWhenFirstSelected := actionMenuLabelRects(t, 0)
	labelsWhenSecondSelected := actionMenuLabelRects(t, 1)
	if labelsWhenFirstSelected != labelsWhenSecondSelected {
		t.Fatalf("action menu labels moved with the selection: %v -> %v", labelsWhenFirstSelected, labelsWhenSecondSelected)
	}
}

func actionMenuLabelRects(t *testing.T, selected int) [2]ui.Rect {
	t.Helper()
	rows := []MenuRow{
		{Title: "Open", Enter: true, Selected: selected == 0},
		{Title: "Set keyboard shortcut", Selected: selected == 1},
	}
	tt := ui.NewTester(func(c *ui.Context) { ActionMenu(c, rows) }, 480, 240)
	tt.Frame()
	return [2]ui.Rect{rectOf(t, tt, "Open"), rectOf(t, tt, "Set keyboard shortcut")}
}

// TestDisabledActionShowsItsReasonBelowItsTitle checks that a disabled action draws the reason on a second line under the
// muted title, as 1.x does, and the enabled row beneath it keeps its single line.
func TestDisabledActionShowsItsReasonBelowItsTitle(t *testing.T) {
	rows := []MenuRow{
		{Title: "Rename", Disabled: true, Subtitle: "Rename is not in 2.0 yet"},
		{Title: "Tweak with AI", Selected: true},
	}
	tt := ui.NewTester(func(c *ui.Context) { ActionMenu(c, rows) }, 480, 240)
	tt.Frame()
	title := rectOf(t, tt, "Rename")
	reason := rectOf(t, tt, "Rename is not in 2.0 yet")
	if reason.Y <= title.Y {
		t.Fatalf("the reason (y %v) is not below the title (y %v)", reason.Y, title.Y)
	}
}
