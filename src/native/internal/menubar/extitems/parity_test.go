package extitems

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// showBars replaces the extension's items with bars and renders them now, as a refresh does.
func (h *harness) showBars(bars ...api.MenuBar) {
	h.t.Helper()
	h.source.set(func(s *fakeSource) { s.bars = bars })
	h.svc.Refresh("acme.menubar")
	h.settle()
}

// rowWith is the menu row with id in the item's menu.
func rowWith(t *testing.T, items []platform.MenuItem, id string) platform.MenuItem {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("no row %q", id)
	return platform.MenuItem{}
}

func TestSubtitleJoinsTheLabelAndShortcutShowsOffLinuxOnly(t *testing.T) {
	bar := api.MenuBar{ID: "main", Icon: "timer", Sections: []api.MenuSection{{Rows: []api.MenuRow{
		{ID: "count", Title: "Count", Subtitle: "last minute", Shortcut: "Command+K", Action: runAction("bump")},
	}}}}
	for _, tc := range []struct{ goos, accelerator string }{{"darwin", "Command+K"}, {"linux", ""}} {
		h := newHarnessFor(t, tc.goos)
		h.showBars(bar)
		row := rowWith(t, h.item("main").Items(), "0.0")
		if row.Label != "Count — last minute" {
			t.Fatalf("%s label = %q, want the subtitle after an em dash", tc.goos, row.Label)
		}
		if row.Accelerator != tc.accelerator {
			t.Fatalf("%s accelerator = %q, want %q", tc.goos, row.Accelerator, tc.accelerator)
		}
	}
}

func TestCheckedRowDrawsACheckbox(t *testing.T) {
	h := newHarness(t)
	h.showBars(api.MenuBar{ID: "main", Sections: []api.MenuSection{{Rows: []api.MenuRow{
		{ID: "on", Title: "On", Checked: api.Checkbox(true), Action: runAction("on")},
		{ID: "off", Title: "Off", Checked: api.Checkbox(false), Action: runAction("off")},
		{ID: "plain", Title: "Plain", Action: runAction("plain")},
	}}}})
	on := rowWith(t, h.item("main").Items(), "0.0")
	if !on.Checkable || !on.Checked {
		t.Fatalf("checked row = %+v, want a ticked checkbox", on)
	}
	if off := rowWith(t, h.item("main").Items(), "0.1"); !off.Checkable || off.Checked {
		t.Fatalf("unchecked row = %+v, want an empty checkbox", off)
	}
	if plain := rowWith(t, h.item("main").Items(), "0.2"); plain.Checkable {
		t.Fatalf("plain row = %+v, want no checkbox", plain)
	}
}

func TestRowIconIsTheGlyphOnMacOSOnly(t *testing.T) {
	bar := api.MenuBar{ID: "main", Sections: []api.MenuSection{{Rows: []api.MenuRow{
		{ID: "bell", Title: "Bell", Icon: "bell", Action: runAction("bell")},
	}}}}
	mac := newHarnessFor(t, "darwin")
	mac.showBars(bar)
	icon := rowWith(t, mac.item("main").Items(), "0.0").Icon
	want := menubar.ChooseGlyph("bell", "darwin", false)
	if icon == nil || !icon.Template || !bytes.Equal(icon.PNG, want.PNG) || len(want.PNG) == 0 {
		t.Fatalf("macOS row icon = %+v, want the bell template", icon)
	}
	linux := newHarnessFor(t, "linux")
	linux.showBars(bar)
	if icon := rowWith(t, linux.item("main").Items(), "0.0").Icon; icon != nil {
		t.Fatal("a Linux row has an icon; 1.x draws row icons on macOS only")
	}
}

func TestBarGlyphFollowsItsNameAndTheBar(t *testing.T) {
	mac := newHarnessFor(t, "darwin")
	tray := mac.item("main")
	icon, template := tray.Icon()
	want := menubar.ChooseGlyph("timer", "darwin", false)
	if !bytes.Equal(icon, want.PNG) || !template {
		t.Fatalf("macOS bar glyph template=%v, want the timer template", template)
	}

	linux := newHarnessFor(t, "linux")
	bar := linux.item("main")
	bar.SetDark(true)
	bar.ChangeAppearance()
	icon, template = bar.Icon()
	white := menubar.ChooseGlyph("timer", "linux", true)
	if !bytes.Equal(icon, white.PNG) || template {
		t.Fatalf("dark Linux bar glyph template=%v, want the white timer", template)
	}
}

func TestOffMacOSTheTitleJoinsTheTooltip(t *testing.T) {
	h := newHarnessFor(t, "linux")
	h.showBars(clockBar("04:59"))
	tray := h.item("main")
	if tray.Title() != "" {
		t.Fatalf("Linux title = %q, want none; the title joins the tooltip", tray.Title())
	}
	if tip := tray.ToolTip(); tip != "Clock — 04:59" {
		t.Fatalf("Linux tooltip = %q, want the title after an em dash", tip)
	}
}

// recordedPopovers is the popover host the tests give the service. It keeps what it was asked, in order.
type recordedPopovers struct {
	mu        sync.Mutex
	toggles   []string
	refreshes []string
}

func (r *recordedPopovers) Toggle(extensionID, barID string, _ platform.Tray, _ api.Popover) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.toggles = append(r.toggles, extensionID+"/"+barID)
}

func (r *recordedPopovers) Refresh(extensionID, barID string, _ platform.Tray, _ api.Popover) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refreshes = append(r.refreshes, extensionID+"/"+barID)
}

func (r *recordedPopovers) counts() (toggles, refreshes int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.toggles), len(r.refreshes)
}

// popoverBar is an item with a popover of one list and a menu of one row.
func popoverBar() api.MenuBar {
	bar := clockBar("04:59")
	view := api.ListView("Timer", api.Item{ID: "row", Title: "Remaining"})
	bar.Popover = &api.Popover{View: view, Width: 360, Height: 420}
	return bar
}

func TestPopoverOpensOnClickAndTheMenuOnRightClickOnMacOS(t *testing.T) {
	h := newHarnessFor(t, "darwin")
	h.showBars(popoverBar())
	tray := h.item("main")
	if len(tray.Items()) != 0 {
		t.Fatalf("a popover item attaches no menu, so the click can open it; it has %d rows", len(tray.Items()))
	}
	tray.Click()
	if toggles, _ := h.popovers.counts(); toggles != 1 {
		t.Fatalf("click toggled the popover %d times, want 1", toggles)
	}
	tray.RightClick()
	popUps := tray.PopUps()
	if len(popUps) != 1 || menuLabelsOf(popUps[0]) != "Counter|Count 1|Bump|-|More" {
		t.Fatalf("right click showed %v, want the menu rows", popUps)
	}
}

func TestPopoverOnLinuxOpensFromTheMenuFirstRow(t *testing.T) {
	h := newHarnessFor(t, "linux")
	h.showBars(popoverBar())
	tray := h.item("main")
	tray.Click()
	if toggles, _ := h.popovers.counts(); toggles != 0 {
		t.Fatalf("Linux trays report no clicks, but the click toggled the popover %d times", toggles)
	}
	if first := tray.Items()[0]; first.Label != "Open Clock" {
		t.Fatalf("first Linux row = %q, want Open Clock", first.Label)
	}
	if err := tray.Invoke("popover-open"); err != nil {
		t.Fatalf("Invoke popover-open: %v", err)
	}
	if toggles, _ := h.popovers.counts(); toggles != 1 {
		t.Fatalf("the Open row toggled the popover %d times, want 1", toggles)
	}
}

func TestOpenPopoverIsRefreshedAfterEveryRender(t *testing.T) {
	h := newHarness(t)
	h.showBars(popoverBar())
	_, before := h.popovers.counts()
	h.svc.Refresh("acme.menubar")
	h.settle()
	if _, after := h.popovers.counts(); after <= before {
		t.Fatalf("refreshes = %d after a render, want more than %d", after, before)
	}
}

// menuLabelsOf joins the labels of rows, dividers as "-", for one comparison.
func menuLabelsOf(rows []platform.MenuItem) string {
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Separator {
			labels = append(labels, "-")
			continue
		}
		labels = append(labels, row.Label)
	}
	return strings.Join(labels, "|")
}
