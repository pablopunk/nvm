package windows

import (
	"strings"
	"testing"
)

func TestMoveFormPlacesTheWindowAtTheTypedOffsets(t *testing.T) {
	d := newDesk(t)
	rows := exactRows(d.deps, moveForm, "120 80")
	if len(rows) != 1 || rows[0].Title != "Move Window: X 120 pt, Y 80 pt" {
		t.Fatalf("rows = %v", rowTitles(rows))
	}
	rows[0].Actions[0].Run(&recordingHost{})
	window, _ := d.windows.Window(d.key)
	if window.Frame.X != 120 || window.Frame.Y != 80 {
		t.Errorf("origin = (%v, %v), want (120, 80)", window.Frame.X, window.Frame.Y)
	}
}

func TestBlankFieldKeepsTheWindowsCurrentValue(t *testing.T) {
	d := newDesk(t)
	rows := exactRows(d.deps, moveForm, "- 40")
	if rows[0].Title != "Move Window: X keeps current, Y 40 pt" {
		t.Fatalf("title = %q", rows[0].Title)
	}
	rows[0].Actions[0].Run(&recordingHost{})
	window, _ := d.windows.Window(d.key)
	if window.Frame.X != 100 || window.Frame.Y != 40 {
		t.Errorf("frame = %+v, want x unchanged at 100 and y 40", window.Frame)
	}
}

func TestResizeFormStartsInPercentAndAcceptsAUnitWord(t *testing.T) {
	d := newDesk(t)
	if rows := exactRows(d.deps, resizeForm, "50 50"); rows[0].Title != "Resize Window: Width 50%, Height 50%" {
		t.Errorf("percent default = %q", rows[0].Title)
	}
	if rows := exactRows(d.deps, resizeForm, "300 200 pt"); rows[0].Title != "Resize Window: Width 300 pt, Height 200 pt" {
		t.Errorf("points word = %q", rows[0].Title)
	}
}

func TestExactFormRefusesOutOfRangeValuesWithTheFieldsCopy(t *testing.T) {
	d := newDesk(t)
	rows := exactRows(d.deps, resizeForm, "0 50")
	if len(rows[0].Actions) != 0 {
		t.Error("a refused size offers an action")
	}
	if rows[0].Title != "Enter a number above 0, up to 100%." {
		t.Errorf("message = %q", rows[0].Title)
	}
}

func TestExactFormAsksForTwoValues(t *testing.T) {
	d := newDesk(t)
	for _, text := range []string{"", "120", "1 2 3 4"} {
		rows := exactRows(d.deps, moveForm, text)
		if len(rows) != 1 || !strings.HasPrefix(rows[0].Title, "Enter two values") {
			t.Errorf("%q rows = %v", text, rowTitles(rows))
		}
	}
	if rows := exactRows(d.deps, moveForm, "- -"); len(rows) != 1 || rows[0].Title != "Enter at least one value." {
		t.Errorf("both blank rows = %v", rowTitles(rows))
	}
}

func TestExactEntriesOpenTheirForms(t *testing.T) {
	d := newDesk(t)
	host := &recordingHost{}
	entryByID(t, Entries(d.deps), moveEntryID).Run(host)
	if len(host.pushed) != 1 {
		t.Fatalf("pushed %d views, want the move form", len(host.pushed))
	}
}
