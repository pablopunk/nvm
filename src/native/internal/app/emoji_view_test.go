package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/emoji"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// emojiGlyphs is the set of every glyph the catalog holds, for counting the cells a frame draws.
func emojiGlyphs() map[string]bool {
	glyphs := map[string]bool{}
	for _, e := range emojiCatalog.All() {
		glyphs[e.Glyph()] = true
	}
	return glyphs
}

// openEmojiPicker shows the picker over the root with an empty query, as the root's Emoji & Symbols row does.
func openEmojiPicker(p *App) {
	goldenOpen(p, p.emojiView())
}

// TestEmojiSearchShowsTheFireEmojiFirst searches the catalog by name and expects the fire character on top.
func TestEmojiSearchShowsTheFireEmojiFirst(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	goldenQuery(p, "fire")
	rows := p.Results()
	if len(rows) == 0 || rows[0].Title != "Fire" || rows[0].Glyph != "🔥" {
		t.Fatalf("top rows = %v, want Fire first", rowTitles(p))
	}
}

// TestEmojiCopyPutsTheGlyphOnTheClipboard runs Copy on the fire row and reads the clipboard back.
func TestEmojiCopyPutsTheGlyphOnTheClipboard(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	goldenQuery(p, "fire")
	actionTitled(t, itemTitled(t, p, "Fire"), "Copy Fire").Run(p)
	got, _ := p.ports.Clipboard.ReadText()
	if got != "🔥" {
		t.Fatalf("clipboard = %q, want the fire glyph", got)
	}
}

// TestEmojiCopyUnicodeCopiesTheCodepoints copies the code points of the fire character, as 1.x does.
func TestEmojiCopyUnicodeCopiesTheCodepoints(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	goldenQuery(p, "fire")
	actionTitled(t, itemTitled(t, p, "Fire"), "Copy Unicode").Run(p)
	got, _ := p.ports.Clipboard.ReadText()
	if got != "U+1F525" {
		t.Fatalf("clipboard = %q, want U+1F525", got)
	}
}

// TestEmojiSkinToneAppliesToTheWavingHand picks a tone from the action menu of the waving hand and checks that
// the grid shows the toned glyph, and that the choice is remembered in the settings.
func TestEmojiSkinToneAppliesToTheWavingHand(t *testing.T) {
	p := goldenPalette(t)
	win := &queuedWindow{}
	p.Attach(win)
	openEmojiPicker(p)
	goldenQuery(p, "waving hand")
	hand := itemTitled(t, p, "Waving Hand")
	actionTitled(t, hand, "Skin Tone…").Run(p)

	tones := p.Results()
	if len(tones) != 6 || tones[5].Title != "Dark" {
		t.Fatalf("skin tone rows = %v, want six tones ending in Dark", rowTitles(p))
	}
	tones[5].Actions[0].Run(p)
	win.drain()

	if p.S.EmojiSkinTone != int(emoji.SkinToneDark) {
		t.Fatalf("remembered tone = %d, want dark", p.S.EmojiSkinTone)
	}
	goldenQuery(p, "waving hand")
	if got := itemTitled(t, p, "Waving Hand").Glyph; got != "👋🏿" {
		t.Fatalf("glyph = %q, want the dark waving hand", got)
	}
}

// TestEmojiEmptyQueryKeepsTheCatalogOrderAfterCopies copies two characters and checks that a new picker still lists the
// catalog from its first entry, as 1.x does: copying does not move a character to the top.
func TestEmojiEmptyQueryKeepsTheCatalogOrderAfterCopies(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	goldenQuery(p, "fire")
	actionTitled(t, itemTitled(t, p, "Fire"), "Copy Fire").Run(p)
	openEmojiPicker(p)
	goldenQuery(p, "grinning")
	actionTitled(t, itemTitled(t, p, "Grinning Face"), "Copy Grinning Face").Run(p)

	openEmojiPicker(p)
	rows := p.Results()
	if len(rows) < 2 || rows[0].Title != "Grinning Face" || rows[1].Title != "Grinning Face With Big Eyes" {
		t.Fatalf("top rows = %v, want the catalog order from Grinning Face", rowTitles(p))
	}
}

// TestEmojiKeysMoveTheChoiceAcrossTheGrid walks the cells with the arrows, Home, End and the page keys, and checks
// the cursor after each key.
func TestEmojiKeysMoveTheChoiceAcrossTheGrid(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	last := len(p.Results()) - 1

	steps := []struct {
		key  ui.Key
		want int
	}{
		{ui.KeyRight, 1},
		{ui.KeyDown, 9},
		{ui.KeyLeft, 8},
		{ui.KeyUp, 0},
		{ui.KeyEnd, last},
		{ui.KeyHome, 0},
		{ui.KeyPageDown, 40},
		{ui.KeyPageUp, 0},
	}
	for _, step := range steps {
		tt.Key(0, step.key)
		tt.Frame()
		if got := p.Chosen(); got != step.want {
			t.Fatalf("after key %v the choice is %d, want %d", step.key, got, step.want)
		}
	}
}

// TestEmojiAnnouncesTheNameOfTheSelectedCell checks the screen reader contract (J27): moving the choice announces
// the name of the cell it lands on.
func TestEmojiAnnouncesTheNameOfTheSelectedCell(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	next := p.Results()[1].Title
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	announced := tt.Announcements()
	if !contains(announced, next) {
		t.Fatalf("announcements = %v, want %q", announced, next)
	}
}

// TestEmojiGridShowsTheFirstMatchesAsThe1xGridDoes checks the 1.x cap: an empty query lists the first 160 characters
// of the catalog, and the rest of the catalog is reached by searching.
func TestEmojiGridShowsTheFirstMatchesAsThe1xGridDoes(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	if n := len(p.Results()); n != 160 {
		t.Fatalf("the picker holds %d entries, want the 160 that 1.x shows", n)
	}
	if n := len(emojiCatalog.All()); n <= 3000 {
		t.Fatalf("the catalog holds %d entries, want more than 3000", n)
	}
}

// TestEmojiGridBuildsOnlyTheCellsOnScreen renders the first matches and checks that the frame draws a bounded number
// of cells: the grid is virtualized, so the row count does not cost a frame.
func TestEmojiGridBuildsOnlyTheCellsOnScreen(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	glyphs := emojiGlyphs()
	drawn := 0
	for _, text := range tt.Texts() {
		if glyphs[text] {
			drawn++
		}
	}
	if drawn == 0 || drawn > 120 {
		t.Fatalf("the frame drew %d cells, want a bounded count of visible cells", drawn)
	}
}

// TestEmojiNoResultsShowsTheEmptyState searches for nothing and expects the 1.x empty state.
func TestEmojiNoResultsShowsTheEmptyState(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	goldenQuery(p, "zzqx")
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	if !tt.HasText("No matching characters") {
		t.Fatal("the picker has no empty state for a query with no match")
	}
}

// TestEmojiTabCyclesTheCategories checks that Tab narrows the grid to the first category, capped as 1.x caps it, and
// Shift+Tab returns to all.
func TestEmojiTabCyclesTheCategories(t *testing.T) {
	p := goldenPalette(t)
	win := &queuedWindow{}
	p.Attach(win)
	openEmojiPicker(p)
	all := len(p.Results())
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()

	tt.Key(0, ui.KeyTab)
	win.drain()
	tt.Frame()
	narrowed := len(p.Results())
	firstCategory := len(emojiCatalog.Search("", emoji.Options{Category: emojiCategoryNames[1]}))
	if want := min(firstCategory, emojiRowLimit); narrowed == 0 || narrowed != want {
		t.Fatalf("after Tab the grid holds %d entries, want %d of the first category", narrowed, want)
	}

	tt.Key(ui.Shift, ui.KeyTab)
	win.drain()
	tt.Frame()
	if got := len(p.Results()); got != all {
		t.Fatalf("after Shift+Tab the grid holds %d entries, want %d", got, all)
	}
}

// contains reports whether want is one of values.
func contains(values []string, want string) bool {
	for _, v := range values {
		if strings.EqualFold(v, want) {
			return true
		}
	}
	return false
}

// TestEmojiEnterPastesWhenThePlatformCanPaste: with paste available, the first action is Paste and Copy is Ctrl+Enter,
// as 1.x lists them.
func TestEmojiEnterPastesWhenThePlatformCanPaste(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	goldenQuery(p, "fire")
	fire := itemTitled(t, p, "Fire")
	if fire.Actions[0].Title != "Paste Fire" || fire.Actions[1].Title != "Copy Fire" || fire.Actions[1].Label != "⌃↵" {
		t.Fatalf("actions = %v", actionTitles(fire.Actions))
	}
}

// TestEmojiEnterCopiesWhenThePlatformCannotPaste: without the paste capability, Copy is the first action and no
// paste is offered.
func TestEmojiEnterCopiesWhenThePlatformCannotPaste(t *testing.T) {
	p := goldenPalette(t)
	var withoutPaste []platform.Capability
	for _, c := range platform.AllCapabilities() {
		if c != platform.CapabilityClipboardPaste {
			withoutPaste = append(withoutPaste, c)
		}
	}
	p.ports.Capabilities = platform.NewCapabilities(withoutPaste...)
	openEmojiPicker(p)
	goldenQuery(p, "fire")
	fire := itemTitled(t, p, "Fire")
	if fire.Actions[0].Title != "Copy Fire" || slices.Contains(actionTitles(fire.Actions), "Paste Fire") {
		t.Fatalf("actions = %v", actionTitles(fire.Actions))
	}
	if hint := p.Top().Hint; hint != "↵ Copy   ⇥ Category" {
		t.Fatalf("hint = %q", hint)
	}
}
