package notes

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestNoteTitleKeepsAnEmptyNumberedItemAndReadsEscapes: a note started with "1" then "1." then "1. " keeps "1." as its
// title until the item has text, and escaped markdown never shows its backslashes.
func TestNoteTitleKeepsAnEmptyNumberedItemAndReadsEscapes(t *testing.T) {
	cases := map[string]string{
		"1":                 "1",
		"1\\.":              "1.",
		"1. ":               "1.",
		"1.":                "1.",
		"1. Buy milk":       "Buy milk",
		"- ":                untitledTitle,
		"\\# not a heading": "# not a heading",
		"2\\) pick":         "2) pick",
	}
	for markdown, want := range cases {
		if got := noteTitle(api.Note{Markdown: markdown}); got != want {
			t.Errorf("noteTitle(%q) = %q, want %q", markdown, got, want)
		}
	}
}
