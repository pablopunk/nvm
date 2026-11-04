package meaningfiles

import (
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Deps are the providers the view searches with.
type Deps struct {
	// Local is the name search of the Files index for the typed query.
	Local search.Provider
	// Meaning is the decision provider. It answers only for the query the user asked.
	Meaning search.Provider
	// SignedIn reports whether the decision can run. Without it the ask row is not offered.
	SignedIn func() bool
}

// askRowScore ranks the ask row above every name match, so the choice to ask is the first row of the list.
const askRowScore = 1000

// Title and the texts of the view's empty state. The empty state shows before any query, when there is nothing to find.
const (
	Title       = "Find Files by Meaning"
	placeholder = "Describe the file you want"
	emptyTitle  = "Describe the file you want"
	emptySub    = "Type what the file is about. Your recent file names can be checked with AI."
)

// View builds the Find Files by Meaning view over deps.
func View(deps Deps) *palette.View {
	asked := &askedQuery{}
	return &palette.View{
		Title:       Title,
		Placeholder: placeholder,
		EmptyTitle:  emptyTitle,
		EmptySub:    emptySub,
		Providers: func() []search.Provider {
			return []search.Provider{
				deps.Local,
				askProvider{asked: asked, signedIn: deps.SignedIn, askRow: askRowFor(asked)},
				gatedProvider{inner: deps.Meaning, asked: asked},
			}
		},
	}
}

// askRowFor returns the row that records q as asked and refreshes the view, so the decision runs for it.
func askRowFor(asked *askedQuery) func(q string) search.Item {
	return func(q string) search.Item {
		return search.Item{
			ID:       askRowID,
			Kind:     search.KindFiles,
			Title:    "Find more files with AI",
			Subtitle: "Send the names of your most recent files for analysis",
			Icon:     "sparkles",
			Score:    askRowScore,
			Actions: []search.Action{{
				Title:   "Find Files",
				Dismiss: search.DismissNever,
				Run: func(h search.Host) {
					asked.set(q)
					h.Refresh()
				},
			}},
		}
	}
}
