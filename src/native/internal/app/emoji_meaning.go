package app

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/charsearch"
	"github.com/pablopunk/nvm/native/internal/features/emoji"
	"github.com/pablopunk/nvm/native/internal/search"
)

// characterSearch is the meaning-based second batch of the Emoji & Symbols picker: the matcher that answers and
// remembers the searches, and the decision client and typing pause it runs with. The app fills the client from the
// signed-in account; a test replaces the client and the pause.
type characterSearch struct {
	matcher *charsearch.Matcher
	decide  func() charsearch.Decider
	pause   *time.Duration
}

// characters returns the picker's meaning search, created on first use on the UI thread.
func (p *App) characters() *characterSearch {
	if p.characterSearch == nil {
		p.characterSearch = &characterSearch{}
	}
	if p.characterSearch.matcher == nil {
		p.characterSearch.matcher = charsearch.NewMatcher(characterRecords())
	}
	return p.characterSearch
}

// characterRecords is the emoji catalog as the matcher reads it, in catalog order.
func characterRecords() []charsearch.Record {
	entries := emojiCatalog.All()
	records := make([]charsearch.Record, 0, len(entries))
	for _, e := range entries {
		records = append(records, charsearch.Record{Key: e.ID(), Glyph: e.Glyph(), Name: e.Title(), Group: e.Category()})
	}
	return records
}

// emojiProviders are the sources of the picker's rows for a query: the catalog's own matches, then, when the account
// can ask the model, the meaning matches for the same query.
func (p *App) emojiProviders(category string, tone emoji.SkinTone) []search.Provider {
	providers := []search.Provider{search.NewRowsProvider("emoji", p.emojiRows(category, tone))}
	if meaning := p.emojiMeaningProvider(category, tone); meaning != nil {
		providers = append(providers, meaning)
	}
	return providers
}

// emojiMeaningProvider is the second batch for the picker's text, or nil while the account is signed out.
func (p *App) emojiMeaningProvider(category string, tone emoji.SkinTone) search.Provider {
	decider := p.characterDecider()
	if decider == nil {
		return nil
	}
	return charsearch.NewProvider(charsearch.Options{
		Decider:  decider,
		Matcher:  p.characters().matcher,
		Category: category,
		Clock:    p.ports.Clock,
		Pause:    p.characterPause(),
		Rows:     func(matches []charsearch.Record) []search.Item { return p.emojiMeaningRows(matches, tone) },
	})
}

// characterDecider is the decision client of the signed-in account, or nil. The nil is untyped, so the provider is
// absent rather than asking a client that cannot answer.
func (p *App) characterDecider() charsearch.Decider {
	if decide := p.characters().decide; decide != nil {
		return decide()
	}
	client := p.backendClient()
	if client == nil || !p.signedIn() {
		return nil
	}
	return client
}

// characterPause is how long the picker's text must rest before the model is asked.
func (p *App) characterPause() time.Duration {
	if pause := p.characters().pause; pause != nil {
		return *pause
	}
	return charsearch.TypingPause
}

// emojiMeaningRows are the rows of the model's matches, best first, with the skin tone the picker shows. They score
// zero, so the catalog's own matches stay above them.
func (p *App) emojiMeaningRows(matches []charsearch.Record, tone emoji.SkinTone) []search.Item {
	items := make([]search.Item, 0, len(matches))
	for _, m := range matches {
		if e, ok := emojiCatalog.Lookup(m.Key); ok {
			items = append(items, p.emojiItem(e, tone, 0))
		}
	}
	return items
}
