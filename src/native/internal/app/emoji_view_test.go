package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/emoji"
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
	actionTitled(t, itemTitled(t, p, "Fire"), "Copy").Run(p)
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

// TestEmojiRecentsLeadTheGridInTheOrderUsed copies two characters and checks that a new picker lists them first,
// newest first.
func TestEmojiRecentsLeadTheGridInTheOrderUsed(t *testing.T) {
	p := goldenPalette(t)
	openEmojiPicker(p)
	goldenQuery(p, "fire")
	actionTitled(t, itemTitled(t, p, "Fire"), "Copy").Run(p)
	openEmojiPicker(p)
	goldenQuery(p, "grinning")
	actionTitled(t, itemTitled(t, p, "Grinning Face"), "Copy").Run(p)

	openEmojiPicker(p)
	rows := p.Results()
	if len(rows) < 2 || rows[0].Title != "Grinning Face" || rows[1].Title != "Fire" {
		t.Fatalf("top rows = %v, want Grinning Face then Fire", rowTitles(p))
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

// TestEmojiArrowsMoveTheCellsWhileTheSearchFieldHasFocus shows the picker as the hotkey does, so the search field has
// the keyboard, and walks the four arrows, first with an empty query and then with a typed one. The arrows move the
// cells and leave the query alone, as 1.x's grid takes them from the field.
func TestEmojiArrowsMoveTheCellsWhileTheSearchFieldHasFocus(t *testing.T) {
	p := goldenPalette(t)
	p.Attach(&queuedWindow{})
	p.Open(p.emojiView())
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()

	steps := []struct {
		key   ui.Key
		delta int
	}{
		{ui.KeyRight, 1},
		{ui.KeyDown, emojiColumns},
		{ui.KeyLeft, -1},
		{ui.KeyUp, -emojiColumns},
	}
	for _, query := range []string{"", "smile"} {
		if query != "" {
			tt.Type(query)
			tt.Frame()
		}
		start := p.Chosen()
		for _, step := range steps {
			tt.Key(0, step.key)
			tt.Frame()
			start += step.delta
			if got := p.Chosen(); got != start {
				t.Fatalf("query %q: after key %v the choice is %d, want %d", query, step.key, got, start)
			}
		}
		if p.Query() != query {
			t.Fatalf("the arrows changed the query to %q, want %q", p.Query(), query)
		}
		p.SetQuery("")
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
