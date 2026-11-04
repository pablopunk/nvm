package app

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/dictation/cleanup"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// dictionaryView edits the custom dictionary as palette rows: the typed text adds a term, and each term row removes
// itself. A term is short, so it never needs an editor.
func (p *App) dictionaryView() *palette.View {
	return &palette.View{
		Title:       "Custom Dictionary",
		Placeholder: "Type a preferred term, then press Enter",
		EmptyTitle:  "No terms yet",
		EmptySub:    "Type a name or spelling the transcripts should keep",
		Search: func(q string) []search.Item {
			var items []search.Item
			if term := strings.TrimSpace(q); term != "" {
				items = append(items, p.dictionaryAddRow(term))
			}
			for _, term := range cleanup.DictionaryTerms(p.dictationSettings().Dictionary) {
				items = append(items, p.dictionaryTermRow(term))
			}
			return items
		},
	}
}

// dictionaryAddRow adds the typed term. A term the dictionary already holds changes nothing.
func (p *App) dictionaryAddRow(term string) search.Item {
	return search.Item{
		Title:    "Add " + term,
		Subtitle: "Keep this spelling in AI cleanup",
		Icon:     "plus",
		Actions: []search.Action{{Title: "Add", Dismiss: search.DismissNever, Run: func(h search.Host) {
			if updated, changed := cleanup.AddTerm(p.dictationSettings().Dictionary, term); changed {
				p.setDictationSettings(func(s *dictationSettings) { s.Dictionary = updated })
			}
			h.Refresh()
		}}},
	}
}

// dictionaryTermRow is one saved term; its action removes it.
func (p *App) dictionaryTermRow(term string) search.Item {
	return search.Item{
		Title:    term,
		Subtitle: "Preferred spelling",
		Icon:     "type",
		Actions: []search.Action{{Title: "Remove", Dismiss: search.DismissNever, Run: func(h search.Host) {
			p.setDictationSettings(func(s *dictationSettings) { s.Dictionary = cleanup.RemoveTerm(s.Dictionary, term) })
			h.Refresh()
		}}},
	}
}
