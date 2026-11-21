package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestSelectionFooterNamesTheChosenTileWithoutTheCategory checks the footer shows the chosen tile's title and subtitle,
// and nothing when no tile is chosen, so it never repeats the category the search accessory shows.
func TestSelectionFooterNamesTheChosenTileWithoutTheCategory(t *testing.T) {
	chosen := search.Item{Title: "grinning face", Subtitle: "U+1F600"}
	if got := selectionFooter(&chosen); got != "grinning face  U+1F600" {
		t.Fatalf("footer = %q, want the tile's title and subtitle", got)
	}
	if got := selectionFooter(nil); got != "" {
		t.Fatalf("footer with no chosen tile = %q, want empty", got)
	}
}
