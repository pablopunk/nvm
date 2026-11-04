package meaningfiles

import (
	"context"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// goldenOptions is the group of the view's goldens under testdata/golden/meaningfiles at the palette's size.
var goldenOptions = golden.Options{Group: "meaningfiles", Width: 720, Height: 470}

// goldenPalette shows the view over deps, typed query first, so the goldens render a settled state.
func goldenPalette(t *testing.T, deps Deps, query string) *palette.Palette {
	t.Helper()
	p, _ := viewWith(t, deps)
	p.SetQuery(query)
	return p
}

// goldenDeps answers the name search with two beach files for any query that names the beach, as the index does for
// file names; other queries find no name.
func goldenDeps(signedIn bool, meaning search.Provider) Deps {
	beach := []search.Item{
		fileRow("/home/a/beach-2024.jpg", "beach-2024.jpg"),
		fileRow("/home/a/beach-notes.txt", "beach-notes.txt"),
	}
	local := search.NewRowsProvider("local", func(_ context.Context, q string) []search.Item {
		if !strings.Contains(q, "beach") {
			return nil
		}
		return beach
	})
	return Deps{Local: local, Meaning: meaning, SignedIn: func() bool { return signedIn }}
}

func TestGoldenEmptyPrompt(t *testing.T) {
	golden.AssertMatrix(t, "empty", goldenPalette(t, goldenDeps(true, fixedProvider("meaning")), "").View, goldenOptions)
}

func TestGoldenAskRowBeforeTheDecision(t *testing.T) {
	golden.AssertMatrix(t, "ask", goldenPalette(t, goldenDeps(true, fixedProvider("meaning")), "beach").View, goldenOptions)
}

func TestGoldenMatchesAfterTheDecision(t *testing.T) {
	deps := goldenDeps(true, fixedProvider("meaning", fileRow("/home/a/sunset.png", "sunset.png")))
	p := goldenPalette(t, deps, "holiday")
	askRow(t, p).Actions[0].Run(&host{p: p})
	golden.AssertMatrix(t, "matches", p.View, goldenOptions)
}

func TestGoldenSignedOutHasNoAsk(t *testing.T) {
	golden.AssertMatrix(t, "signed-out", goldenPalette(t, goldenDeps(false, fixedProvider("meaning")), "beach").View, goldenOptions)
}

func TestGoldenFailedDecisionExplains(t *testing.T) {
	failure := search.Item{ID: "files-meaning-error", Kind: search.KindFiles, Title: "Could not search files by meaning: not enough credits.", Contextual: true}
	deps := goldenDeps(true, fixedProvider("meaning", failure))
	p := goldenPalette(t, deps, "holiday")
	askRow(t, p).Actions[0].Run(&host{p: p})
	golden.AssertMatrix(t, "failed", p.View, goldenOptions)
}
