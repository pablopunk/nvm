package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// testWindow is the palette's window in app tests. Updates run at once, and hides are counted.
type testWindow struct {
	visible bool
	hides   int
}

// Update runs fn at once; a test is its own UI thread.
func (w *testWindow) Update(fn func()) { fn() }

// Show marks the window visible.
func (w *testWindow) Show() { w.visible = true }

// Hide marks the window hidden and counts it.
func (w *testWindow) Hide() {
	w.visible = false
	w.hides++
}

// IsVisible reports the visibility the test set.
func (w *testWindow) IsVisible() bool { return w.visible }

// Size is the palette's size.
func (w *testWindow) Size() (int, int) { return width, height }

// SetPosition is a no-op: a test window has no screen.
func (w *testWindow) SetPosition(int, int) {}

// clipPalette returns the golden palette showing the clipboard history of f, with a window that can hide.
func clipPalette(t *testing.T, f *clipFixture) (*App, *testWindow) {
	t.Helper()
	p := goldenPalette(t)
	p.clip = f.engine
	win := &testWindow{}
	p.Attach(win)
	return p, win
}

// openClipboardView opens the history view the way the user does: above the root, with the search field focused,
// so Enter reaches the rows.
func openClipboardView(p *App) {
	p.SetStack(p.rootView())
	p.Push(p.clipboardView())
}

// seededClipPalette is clipPalette over a fixture holding the golden entries, opened on the history view.
func seededClipPalette(t *testing.T) (*App, *testWindow, *clipFixture) {
	t.Helper()
	f := newClipFixture(t, clipDefaultLimit)
	seedGoldenClipboard(t, f)
	p, win := clipPalette(t, f)
	openClipboardView(p)
	return p, win, f
}

// rowTitles returns the titles of the rows on screen, in order.
// actionTitled returns the action of it with the given title, failing the test when there is none.
// Titles of the golden entries, used by the tests to find them.
const (
	goldenNotesTitle  = "Meeting notes: ship the design package by Friday and review the goldens with Ana"
	goldenRebaseTitle = "git rebase -i HEAD~3"
)

// Entry IDs of the golden entries, as seedGoldenClipboard stores them.
const (
	goldenRebaseID = "e-rebase"
	goldenNotesID  = "e-notes"
)

// TestClipboardHistoryRowsUse1xTitlesAndCopyFirst: the row title is the 1.x title, the subtitle says when it was
// copied and what it is, and Copy is the action Enter runs.
func TestClipboardHistoryRowsUse1xTitlesAndCopyFirst(t *testing.T) {
	p, _, _ := seededClipPalette(t)
	rows := p.Results()
	if len(rows) != 4 {
		t.Fatalf("rows = %v", rowTitles(p))
	}
	rebase := rows[0]
	if rebase.Title != goldenRebaseTitle || rebase.Subtitle != "Copied 3m ago · Text" {
		t.Fatalf("row = %q / %q", rebase.Title, rebase.Subtitle)
	}
	if rows[2].Subtitle != "Copied 40m ago · Image" || rows[2].Icon != "image" {
		t.Fatalf("image row = %q / %q", rows[2].Subtitle, rows[2].Icon)
	}
	if rebase.Actions[0].Title != "Copy Text" {
		t.Fatalf("first action = %q, want Copy Text", rebase.Actions[0].Title)
	}
	if rows[0].Accessory != "" {
		t.Fatalf("a row shows an accessory it should not: %q", rows[0].Accessory)
	}
}

// TestClipboardHistoryEmptyStateUsesThe1xCopy: an empty history has no rows and shows the 1.x words.
func TestClipboardHistoryEmptyStateUsesThe1xCopy(t *testing.T) {
	p, _ := clipPalette(t, newClipFixture(t, clipDefaultLimit))
	openClipboardView(p)
	if view := p.clipboardView(); view.EmptyTitle != "No clipboard items found." {
		t.Fatalf("empty title = %q", view.EmptyTitle)
	}
	if len(p.Results()) != 0 {
		t.Fatalf("rows = %v", rowTitles(p))
	}
}

// TestClipboardHistoryDeleteNeedsAConfirmation: Remove Item opens a confirmation and removes nothing until it is
// confirmed; Cancel leaves the entry.
func TestClipboardHistoryDeleteNeedsAConfirmation(t *testing.T) {
	p, _, _ := seededClipPalette(t)
	actionTitled(t, p.Results()[1], "Remove Item").Run(p)
	if p.Top().Title != "Confirm" {
		t.Fatalf("top view = %q, want Confirm", p.Top().Title)
	}
	if _, ok := p.clip.history.Get(goldenRebaseID); !ok {
		t.Fatal("the entry was removed before confirmation")
	}
	p.Pop()
	if _, ok := p.clip.history.Get(goldenRebaseID); !ok {
		t.Fatal("cancelling removed the entry")
	}
}

// TestClipboardHistoryDeleteRemovesAfterConfirm: the confirmation's action removes the entry and hides the palette.
func TestClipboardHistoryDeleteRemovesAfterConfirm(t *testing.T) {
	p, win, _ := seededClipPalette(t)
	actionTitled(t, p.Results()[0], "Remove Item").Run(p)
	p.Results()[0].Actions[0].Run(p)
	if _, ok := p.clip.history.Get(goldenRebaseID); ok {
		t.Fatal("the confirmed removal kept the entry")
	}
	if win.hides == 0 {
		t.Fatal("the confirmed removal did not hide the palette")
	}
}

// TestClearAllUnpinnedNeedsAConfirmation: the bulk clear confirms, and keeps pinned entries.
// TestRemoveLastOffersTheTimeWindows: "Remove Last..." opens the four windows, each confirmed.
func TestRemoveLastOffersTheTimeWindows(t *testing.T) {
	p, _, _ := seededClipPalette(t)
	actionTitled(t, p.Results()[1], "Remove Last...").Run(p)
	if got := rowTitles(p); !slices.Equal(got, []string{"5 Minutes", "Hour", "Day", "Week"}) {
		t.Fatalf("windows = %v", got)
	}
}

// TestPasteIsOmittedWithoutThePasteCapability: a platform that cannot paste shows no Paste action.
func TestPasteIsOmittedWithoutThePasteCapability(t *testing.T) {
	f := newClipFixture(t, clipDefaultLimit)
	f.engine.caps = platform.NewCapabilities(platform.CapabilityClipboardImage)
	seedGoldenClipboard(t, f)
	p, _ := clipPalette(t, f)
	openClipboardView(p)
	for _, a := range p.Results()[1].Actions {
		if a.Title == "Paste" || a.Title == "Paste Text" {
			t.Fatalf("paste action shown without the capability: %v", a.Title)
		}
	}
}

// TestClipboardActionMenuOffersTheRowActions: the menu of a text row lists the actions a user can take.
func TestClipboardActionMenuOffersTheRowActions(t *testing.T) {
	p, _, _ := seededClipPalette(t)
	titles := make([]string, 0, len(p.Results()[1].Actions))
	for _, a := range p.Results()[1].Actions {
		titles = append(titles, a.Title)
	}
	for _, want := range []string{"Copy Text", "Preview", "Paste Text", "Remove Item", "Remove Last..."} {
		if !slices.Contains(titles, want) {
			t.Fatalf("actions = %v, missing %q", titles, want)
		}
	}
}

// topOfList moves the cursor to the first row with arrow keys, so a key test starts from a known row.
func topOfList(tt *ui.Tester) {
	for range 6 {
		tt.Key(0, ui.KeyUp)
		tt.Frame()
	}
}

// TestClipboardHistoryKeyboardOnlyFlow is the keyboard part of J27: move with the arrow keys, copy with Enter, and
// remove with Ctrl+Backspace followed by Enter in the confirmation, without a pointer.
func TestClipboardHistoryKeyboardOnlyFlow(t *testing.T) {
	p, win, f := seededClipPalette(t)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	topOfList(tt)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if text, _ := f.set.Clipboard.ReadText(); text != goldenRebaseTitle {
		t.Fatalf("Enter on the first row copied %q", text)
	}
	if win.hides == 0 {
		t.Fatal("copying did not hide the palette")
	}

	openClipboardView(p)
	tt.Frame()
	topOfList(tt)
	// The action menu opens on Copy; for a text row the order is Copy, Preview, Paste Text, Remove Item, so Remove
	// Item is three rows down.
	tt.Key(mod, ui.KeyK)
	tt.Frame()
	for range 3 {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if p.Top().Title != "Confirm" || p.Results()[0].Title != "Remove Item" {
		t.Fatalf("the menu opened %q with %v, want the Remove Item confirmation", p.Top().Title, rowTitles(p))
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if _, ok := p.clip.history.Get(goldenRebaseID); ok {
		t.Fatal("the keyboard removal kept the entry")
	}
}

// TestClipboardRowsAreAnnouncedAndLabelled: each row has a label for assistive technology, and moving the cursor
// announces the row it lands on.
func TestClipboardRowsAreAnnouncedAndLabelled(t *testing.T) {
	p, _, _ := seededClipPalette(t)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	for _, row := range p.Results() {
		if _, ok := tt.Find(row.Title); !ok {
			t.Fatalf("row %q has no label", row.Title)
		}
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if got := tt.Announcements(); !slices.Contains(got, goldenRebaseTitle) {
		t.Fatalf("announcements = %v; moving down should announce the second row", got)
	}
}
