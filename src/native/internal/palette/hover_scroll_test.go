package palette

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func hoverScrollNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("Item %02d", i)
	}
	return names
}

// scrollSnapshot is what shows the list moved: the rows it shows and where one row sits.
func scrollSnapshot(h *harness, anchor string) string {
	first, last := h.p.list.Visible()
	rect, _ := h.tt.Find(anchor)
	return fmt.Sprintf("rows %d-%d, %s at y=%.1f", first, last, anchor, rect.Y)
}

func movePointerOver(h *harness, title string) {
	rect, ok := h.tt.Find(title)
	if !ok {
		h.t.Fatalf("row %q is not on screen", title)
	}
	h.tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	h.frame()
	h.frame()
}

// TestHoveringTheLastVisibleRowNeverScrollsTheList checks the pointer choosing the row at the bottom edge leaves the
// scroll offset alone, however long it rests there.
func TestHoveringTheLastVisibleRowNeverScrollsTheList(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("List", &ran, hoverScrollNames(24)...), true)
	movePointerOver(h, "Item 00")
	before := scrollSnapshot(h, "Item 00")
	_, last := h.p.list.Visible()

	movePointerOver(h, fmt.Sprintf("Item %02d", last))
	for range 5 {
		h.frame()
	}
	if got := scrollSnapshot(h, "Item 00"); got != before {
		t.Fatalf("hovering the last visible row scrolled the list: %s, was %s", got, before)
	}
	if h.p.Chosen() != last {
		t.Fatalf("chosen = %d, want the hovered row %d", h.p.Chosen(), last)
	}
}

// TestHoveringTheFirstVisibleRowNeverScrollsTheList checks the same at the top edge of a list scrolled down.
func TestHoveringTheFirstVisibleRowNeverScrollsTheList(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("List", &ran, hoverScrollNames(24)...), true)
	for range 12 {
		h.key(0, ui.KeyDown)
	}
	h.frame()
	first, _ := h.p.list.Visible()
	if first == 0 {
		t.Fatal("the arrows did not scroll the list")
	}
	title := fmt.Sprintf("Item %02d", first)
	before := scrollSnapshot(h, "Item 12")

	movePointerOver(h, "Item 12")
	movePointerOver(h, title)
	for range 5 {
		h.frame()
	}
	if got := scrollSnapshot(h, "Item 12"); got != before {
		t.Fatalf("hovering the first visible row scrolled the list: %s, was %s", got, before)
	}
}

// TestArrowKeysStillScrollTheChosenRowIntoView checks keyboard movement past the last visible row scrolls the list.
func TestArrowKeysStillScrollTheChosenRowIntoView(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("List", &ran, hoverScrollNames(24)...), true)
	for range 20 {
		h.key(0, ui.KeyDown)
	}
	h.frame()
	first, last := h.p.list.Visible()
	if first == 0 || h.p.Chosen() < first || h.p.Chosen() > last {
		t.Fatalf("chosen %d is not in view %d-%d after arrowing down", h.p.Chosen(), first, last)
	}
	for range 20 {
		h.key(0, ui.KeyUp)
	}
	h.frame()
	if first, _ := h.p.list.Visible(); first != 0 {
		t.Fatalf("first visible row = %d after arrowing back to the top, want 0", first)
	}
}

// TestHoveredRowDrawsNoSecondPlateBesideTheCursor checks that a row the pointer rests on while the keyboard moved
// elsewhere is not drawn highlighted, so only the cursor row carries a plate.
func TestHoveredRowDrawsNoSecondPlateBesideTheCursor(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("List", &ran, hoverScrollNames(6)...), true)
	movePointerOver(h, "Item 00")
	movePointerOver(h, "Item 02")
	h.key(0, ui.KeyDown)
	h.frame()
	if h.p.Chosen() != 3 {
		t.Fatalf("chosen = %d, want 3", h.p.Chosen())
	}
	hovered, _ := h.tt.Find("Item 02")
	resting, _ := h.tt.Find("Item 04")
	img := h.tt.Image()
	at := func(r ui.Rect) any { return img.At(int(r.X+r.W*0.8), int(r.Y+r.H/2)) }
	if at(hovered) != at(resting) {
		t.Fatalf("a row the pointer rests on is drawn %v, a plain row %v: two highlights beside the cursor", at(hovered), at(resting))
	}
	if chosen, _ := h.tt.Find("Item 03"); at(chosen) == at(resting) {
		t.Fatal("the chosen row is drawn like a plain row, so the comparison proves nothing")
	}
}
