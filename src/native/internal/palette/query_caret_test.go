package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestReplacedQueryTakesTheCaretToItsEnd sets the search text the way a form field's edit does and types on it. The
// caret must sit after the text, so the typed runes append to it instead of landing before it.
func TestReplacedQueryTakesTheCaretToItsEnd(t *testing.T) {
	root := &View{Title: "Root", Search: func(string) []search.Item { return nil }}
	h := newHarness(t, root, true)

	h.p.SetQuery("A")
	h.frame()
	h.typeText("da Lovelace")
	if got := h.p.Query(); got != "Ada Lovelace" {
		t.Fatalf("query = %q; want the typed text after the replaced one", got)
	}
}

// TestReplacedQueryDeletesFromItsEnd checks Backspace after a replacement: it erases the end of the text, so a
// corrected value can be typed over a rejected one.
func TestReplacedQueryDeletesFromItsEnd(t *testing.T) {
	root := &View{Title: "Root", Search: func(string) []search.Item { return nil }}
	h := newHarness(t, root, true)

	h.p.SetQuery("not-an-email")
	h.frame()
	for range len("not-an-email") {
		h.key(0, ui.KeyBackspace)
	}
	h.typeText("ada@example.test")
	if got := h.p.Query(); got != "ada@example.test" {
		t.Fatalf("query = %q; want the corrected text alone", got)
	}
}
