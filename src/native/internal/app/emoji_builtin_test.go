package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	builtin "github.com/pablopunk/nvm/native/internal/ext/builtins/emoji"
	"github.com/pablopunk/nvm/native/internal/features/emoji"
)

// openEmojiBuiltIn runs the Emoji & Symbols command of the palette and waits until the built-in's grid shows its
// characters. The command runs off the test's thread, so the window's queue is drained until the grid arrives.
func openEmojiBuiltIn(t *testing.T, p *App) *queuedWindow {
	t.Helper()
	win := &queuedWindow{}
	win.Show()
	p.Attach(win)
	p.uiQueue = win
	goldenQuery(p, "emoji")
	itemTitled(t, p, builtin.Title).Actions[0].Run(p)
	deadline := time.Now().Add(10 * time.Second)
	for {
		win.drain()
		if rows := p.Results(); len(rows) > 0 && rows[0].Glyph != "" {
			return win
		}
		if time.Now().After(deadline) {
			t.Fatal("the Emoji & Symbols grid never opened")
		}
		time.Sleep(time.Millisecond)
	}
}

// awaitRows drains the window until the palette shows n rows, as the command's answer arrives off the test's thread.
func awaitRows(t *testing.T, p *App, win *queuedWindow, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for len(p.Results()) != n {
		win.drain()
		if time.Now().After(deadline) {
			t.Fatalf("the palette shows %d rows, want %d", len(p.Results()), n)
		}
		time.Sleep(time.Millisecond)
	}
	win.drain()
}

// hasGlyph reports whether the palette's rows hold the titled row with the glyph.
func hasGlyph(p *App, title, glyph string) bool {
	for _, row := range p.Results() {
		if row.Title == title && row.Glyph == glyph {
			return true
		}
	}
	return false
}

// TestEmojiSearchShowsTheFireEmojiFirst searches the catalog by name and expects the fire character on top.
func TestEmojiSearchShowsTheFireEmojiFirst(t *testing.T) {
	p := goldenPalette(t)
	openEmojiBuiltIn(t, p)
	goldenQuery(p, "fire")
	rows := p.Results()
	if len(rows) == 0 || rows[0].Title != "Fire" || rows[0].Glyph != "🔥" {
		t.Fatalf("top rows = %v, want Fire first", rowTitles(p))
	}
}

// TestEmojiCopyPutsTheGlyphOnTheClipboard runs Copy on the fire row and reads the clipboard back.
func TestEmojiCopyPutsTheGlyphOnTheClipboard(t *testing.T) {
	p := goldenPalette(t)
	openEmojiBuiltIn(t, p)
	goldenQuery(p, "fire")
	actionTitled(t, itemTitled(t, p, "Fire"), "Copy Fire").Run(p)
	if got, _ := p.ports.Clipboard.ReadText(); got != "🔥" {
		t.Fatalf("clipboard = %q, want the fire glyph", got)
	}
}

// TestEmojiCopyUnicodeCopiesTheCodepoints copies the code points of the fire character, as 1.x does.
func TestEmojiCopyUnicodeCopiesTheCodepoints(t *testing.T) {
	p := goldenPalette(t)
	openEmojiBuiltIn(t, p)
	goldenQuery(p, "fire")
	actionTitled(t, itemTitled(t, p, "Fire"), "Copy Unicode").Run(p)
	if got, _ := p.ports.Clipboard.ReadText(); got != "U+1F525" {
		t.Fatalf("clipboard = %q, want U+1F525", got)
	}
}

// TestEmojiSkinToneAppliesToTheWavingHand picks a tone from the action menu of the waving hand and checks that the grid
// shows the toned glyph, and that the choice is remembered.
func TestEmojiSkinToneAppliesToTheWavingHand(t *testing.T) {
	p := goldenPalette(t)
	win := openEmojiBuiltIn(t, p)
	goldenQuery(p, "waving hand")
	actionTitled(t, itemTitled(t, p, "Waving Hand"), "Skin Tone…").Run(p)
	awaitRows(t, p, win, 6)
	tones := p.Results()
	if len(tones) != 6 || tones[5].Title != "Dark" {
		t.Fatalf("skin tone rows = %v, want six tones ending in Dark", rowTitles(p))
	}
	tones[5].Actions[0].Run(p)
	deadline := time.Now().Add(10 * time.Second)
	for !hasGlyph(p, "Waving Hand", "👋🏿") {
		if time.Now().After(deadline) {
			t.Fatalf("the grid never showed the dark waving hand, rows %v", rowTitles(p))
		}
		win.drain()
		goldenQuery(p, "waving hand")
		time.Sleep(time.Millisecond)
	}
}

// libraryOptions are the search options of one category, up to the grid's limit.
func libraryOptions(category string) emoji.Options {
	return emoji.Options{Category: category, Limit: builtin.RowLimit}
}

// TestEmojiTabCyclesTheCategories steps the categories with Tab and back with Shift+Tab, and checks the grid holds the
// first category after Tab and every character after Shift+Tab.
func TestEmojiTabCyclesTheCategories(t *testing.T) {
	p := goldenPalette(t)
	win := openEmojiBuiltIn(t, p)
	all := len(p.Results())
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	win.drain()
	tt.Frame()
	library := emoji.NewCatalog()
	firstCategory := len(library.Search("", libraryOptions(library.Categories()[0])))
	if narrowed, want := len(p.Results()), min(firstCategory, builtin.RowLimit); narrowed != want {
		t.Fatalf("after Tab the grid holds %d entries, want %d of the first category", narrowed, want)
	}
	tt.Key(ui.Shift, ui.KeyTab)
	win.drain()
	tt.Frame()
	if got := len(p.Results()); got != all {
		t.Fatalf("after Shift+Tab the grid holds %d entries, want %d", got, all)
	}
}
