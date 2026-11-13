package snippets

import (
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// storageKey is the key the snippets are kept under in the extension's storage.
const storageKey = "snippets"

// rawSnippets are the snippets as stored, malformed entries included, so a save never drops what it does not show. They
// are the stored ones once any was saved, and the legacy ones (the snippets document of earlier versions) until then.
func (e Extension) rawSnippets(ctx api.Context) []Snippet {
	var stored []Snippet
	if found, err := ctx.Storage().Get(storageKey, &stored); err == nil && found {
		return stored
	}
	return slices.Clone(e.legacy)
}

// listSnippets returns the snippets the user can paste, in stored order.
func (e Extension) listSnippets(ctx api.Context) []Snippet {
	return usableSnippets(e.rawSnippets(ctx))
}

// createSnippet adds a snippet from the form's input and returns it. A keyword another snippet holds is refused.
func (e Extension) createSnippet(ctx api.Context, in Input) (Snippet, error) {
	clean, err := normalizeInput(in)
	if err != nil {
		return Snippet{}, err
	}
	if err := checkKeyword(e.listSnippets(ctx), clean.Keyword, ""); err != nil {
		return Snippet{}, err
	}
	now := ctx.Clock().Now()
	sn := Snippet{ID: newID(now, e.sequence.Add(1)), Name: clean.Name, Keyword: clean.Keyword, Text: clean.Text, UpdatedAt: now}
	return sn, e.saveSnippets(ctx, append(e.rawSnippets(ctx), sn))
}

// updateSnippet replaces the fields of a snippet. The snippet keeps its own keyword without counting as taken.
func (e Extension) updateSnippet(ctx api.Context, id string, in Input) (Snippet, error) {
	if _, ok := findSnippet(e.listSnippets(ctx), id); !ok {
		return Snippet{}, errNotFound
	}
	clean, err := normalizeInput(in)
	if err != nil {
		return Snippet{}, err
	}
	if err := checkKeyword(e.listSnippets(ctx), clean.Keyword, id); err != nil {
		return Snippet{}, err
	}
	raw := e.rawSnippets(ctx)
	now := ctx.Clock().Now()
	var updated Snippet
	for i := range raw {
		if raw[i].ID == id {
			raw[i].Name, raw[i].Keyword, raw[i].Text, raw[i].UpdatedAt = clean.Name, clean.Keyword, clean.Text, now
			updated = raw[i]
		}
	}
	return updated, e.saveSnippets(ctx, raw)
}

// deleteSnippet removes a snippet. The palette confirms with the user first.
func (e Extension) deleteSnippet(ctx api.Context, id string) error {
	if _, ok := findSnippet(e.listSnippets(ctx), id); !ok {
		return errNotFound
	}
	raw := e.rawSnippets(ctx)
	return e.saveSnippets(ctx, slices.DeleteFunc(raw, func(sn Snippet) bool { return sn.ID == id }))
}

// saveSnippets writes the whole list to the extension's storage. A failed write is returned, never hidden.
func (e Extension) saveSnippets(ctx api.Context, all []Snippet) error {
	return ctx.Storage().Set(storageKey, all)
}

// findSnippet returns the snippet with the id.
func findSnippet(all []Snippet, id string) (Snippet, bool) {
	i := slices.IndexFunc(all, func(sn Snippet) bool { return sn.ID == id })
	if i < 0 {
		return Snippet{}, false
	}
	return all[i], true
}

// matchKeywords answers a root query. exact holds the snippets whose keyword is the query; prefix holds the others whose
// keyword starts with it. A blank query matches nothing, and a snippet with no keyword is found by its name only.
func matchKeywords(all []Snippet, query string) (exact, prefix []Snippet) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, nil
	}
	for _, sn := range all {
		switch {
		case sn.Keyword == "":
		case sn.Keyword == q:
			exact = append(exact, sn)
		case strings.HasPrefix(sn.Keyword, q):
			prefix = append(prefix, sn)
		}
	}
	return exact, prefix
}

// checkKeyword fails when a snippet other than the one with id holds the keyword. An empty keyword is never taken.
func checkKeyword(all []Snippet, keyword, id string) error {
	if keyword == "" {
		return nil
	}
	for _, sn := range all {
		if sn.Keyword == keyword && sn.ID != id {
			return keywordTakenError(keyword, sn.Name)
		}
	}
	return nil
}

// sortedByName returns the snippets in the order the list view shows them: by name, ignoring case.
func sortedByName(all []Snippet) []Snippet {
	out := slices.Clone(all)
	slices.SortStableFunc(out, func(a, b Snippet) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}
