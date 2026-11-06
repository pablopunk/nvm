package palette

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestFloatingFooterKeepsBodyControlsClear(t *testing.T) {
	view := &View{
		Title: "Editor",
		Body: func(c *ui.Context, _ search.Host) {
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				ui.Box(c).Grow(1)
				ui.Text(c, "Bottom control")
			})
		},
		Primary: func() *design.KeyHint { return &design.KeyHint{Title: "Save", Key: "↵"} },
	}
	h := newHarness(t, view, true)
	control, ok := h.tt.Find("Bottom control")
	if !ok {
		t.Fatal("body control is missing")
	}
	footer, ok := h.tt.Find("Save")
	if !ok {
		t.Fatal("footer action is missing")
	}
	if control.Y+control.H > footer.Y {
		t.Fatalf("body control overlaps the footer: control=%v footer=%v", control, footer)
	}
}

func TestSelectedListRowStaysClearOfFloatingFooter(t *testing.T) {
	names := make([]string, 24)
	for i := range names {
		names[i] = fmt.Sprintf("Item %02d", i)
	}
	var ran []string
	h := newHarness(t, twoActionView("List", &ran, names...), true)
	for _, name := range names[1:] {
		h.key(0, ui.KeyDown)
		row, ok := h.tt.Find(name)
		if !ok {
			t.Fatalf("selected row %q is not visible", name)
		}
		footer, ok := h.tt.Find("Primary")
		if !ok {
			t.Fatal("footer action is missing")
		}
		if row.Y+row.H > footer.Y {
			t.Fatalf("selected row %q overlaps the footer: row=%v footer=%v", name, row, footer)
		}
	}
}

func TestRootFooterShowsPendingStatusWithoutIdentity(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprintf("cached=%t", cached), func(t *testing.T) {
			pending := true
			titles := []string{}
			status := "Loading…"
			if cached {
				titles = []string{"Cached row"}
				status = "Refreshing…"
			}
			h := newHarness(t, loadingView(false, &pending, &titles), true)
			if _, ok := h.tt.Find(status); !ok {
				t.Fatalf("root footer lost %q status", status)
			}
			if _, ok := h.tt.Find("Nevermind"); ok {
				t.Fatal("root identity pill was restored")
			}
			pending = false
			h.frame()
			if _, ok := h.tt.Find(status); ok {
				t.Fatalf("completed load kept %q status", status)
			}
		})
	}
}
