//go:build rig

package rigtest

import (
	"strings"
	"testing"
	"time"
)

// The AI journeys ask from the root: Tab on a typed question opens a chat that sends it at once, as the palette does.

const (
	aiQuestion        = "Say hello"
	streamedFirstWord = "Streamed words"
	streamFinalWord   = "FINAL-WORD-MARK"
	streamFrameDelay  = 200 * time.Millisecond
	streamWait        = 10 * time.Second
)

// streamedReply is long enough that a stop lands while frames are still due: about sixteen frames, 3 s at the paced
// rate, and its last word only appears when the reply finishes.
var streamedReply = strings.Repeat("Streamed words arrive one piece at a time. ", 8) + streamFinalWord + "."

// askAIFromRoot types question at the root and presses Tab once the palette holds the question as its query. Tab
// asks AI only from a non-empty root query; sent before the query lands, it moves focus instead.
func askAIFromRoot(t *testing.T, app *App, question string) {
	t.Helper()
	must(t, app.Type(question))
	waitSearchValue(t, app, question)
	must(t, app.Key("Tab"))
}

// paletteShows reports whether any element of the palette carries text.
func paletteShows(t *testing.T, app *App, text string) bool {
	t.Helper()
	tree, err := app.Tree()
	must(t, err)
	for _, node := range tree.Nodes {
		if nodeHasText(node, text) {
			return true
		}
	}
	return false
}
