package commands

import (
	"strings"
	"unicode/utf8"
)

// MaxChunkRunes is the most text one correction call receives. A longer selection is split, so no reply has to
// reproduce more than a page, and a model that loses its place loses only that piece.
const MaxChunkRunes = 1500

// chunkBoundaries are the places a cut is preferred at, strongest first. A cut lands after the boundary.
var chunkBoundaries = []string{"\n\n", "\n", ". ", "? ", "! ", " "}

// SplitForCorrection splits text into pieces of at most limit runes. The pieces concatenate back to text exactly, so
// the corrected pieces join into the corrected selection. A cut falls after the last boundary in the window, and only
// hard-splits when the window has none.
func SplitForCorrection(text string, limit int) []string {
	var pieces []string
	runes := []rune(text)
	for len(runes) > limit {
		cut := cutAfterBoundary(string(runes[:limit]))
		pieces = append(pieces, string(runes[:cut]))
		runes = runes[cut:]
	}
	if len(runes) > 0 {
		pieces = append(pieces, string(runes))
	}
	return pieces
}

// cutAfterBoundary returns the rune count of window up to and including its last preferred boundary, or all of
// window when it has none.
func cutAfterBoundary(window string) int {
	for _, boundary := range chunkBoundaries {
		if at := strings.LastIndex(window, boundary); at > 0 {
			return utf8.RuneCountInString(window[:at]) + utf8.RuneCountInString(boundary)
		}
	}
	return utf8.RuneCountInString(window)
}
