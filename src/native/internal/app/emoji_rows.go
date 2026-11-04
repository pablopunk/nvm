package app

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/emoji"
	"github.com/pablopunk/nvm/native/internal/search"
)

// emojiRows returns the picker's search: the catalog entries that match the query in the category, with the recent
// characters first when the query is empty. Glyphs carry the remembered skin tone. The search runs off the UI thread,
// so the category and the tone are the values captured when the query starts, and it reads only the documents,
// which lock themselves.
func (p *App) emojiRows(category string, tone emoji.SkinTone) search.RowsFunc {
	return func(_ context.Context, q string) []search.Item {
		entries := emojiCatalog.Search(q, emoji.Options{Category: category})
		if q == "" {
			entries = recentFirst(p.recentEntries(category), entries)
		}
		items := make([]search.Item, len(entries))
		for i, e := range entries {
			items[i] = p.emojiItem(e, tone, len(entries)-i)
		}
		return items
	}
}

// recentEntries returns the recent characters of the category, newest first.
func (p *App) recentEntries(category string) []emoji.Entry {
	recent := emojiCatalog.Recents(recentIDSource{app: p}, emojiRecentLimit)
	if category == emoji.AllCategory {
		return recent
	}
	var out []emoji.Entry
	for _, e := range recent {
		if e.Category() == category {
			out = append(out, e)
		}
	}
	return out
}

// recentFirst lists the recent entries first, then the rest in catalog order, each entry once.
func recentFirst(recent, entries []emoji.Entry) []emoji.Entry {
	seen := make(map[string]bool, len(recent))
	out := make([]emoji.Entry, 0, len(recent)+len(entries))
	for _, e := range recent {
		seen[e.ID()] = true
		out = append(out, e)
	}
	for _, e := range entries {
		if !seen[e.ID()] {
			out = append(out, e)
		}
	}
	return out
}

// emojiItem is the row of one character. Enter runs the first action: Paste where the platform can paste, else Copy.
func (p *App) emojiItem(e emoji.Entry, tone emoji.SkinTone, score int) search.Item {
	glyph := e.Glyph()
	if e.SupportsSkinTone() {
		glyph = e.WithSkinTone(tone)
	}
	actions := p.emojiActions(e, glyph)
	return search.Item{
		Kind:     emojiKind(e),
		Title:    e.Title(),
		Subtitle: e.Codepoints(),
		Glyph:    glyph,
		ID:       e.ID(),
		Score:    score,
		Actions:  actions,
	}
}

// emojiActions are the actions of a character in the order the menu shows them, as 1.x builds them. Paste comes first
// and Enter runs it when the platform can paste; Copy is then the second action, on Ctrl+Enter. Without paste, Copy
// is first. Copy Unicode and, for characters with variants, the skin tone follow.
func (p *App) emojiActions(e emoji.Entry, glyph string) []search.Action {
	var actions []search.Action
	if p.canPaste() {
		actions = append(actions, search.Action{Title: "Paste " + e.Title(), Run: func(h search.Host) { p.emitEmoji(h, e, glyph, true) }})
	}
	actions = append(actions, search.Action{Title: "Copy " + e.Title(), Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) { p.emitEmoji(h, e, glyph, false) }})
	if e.SupportsSkinTone() {
		actions = append(actions, search.Action{Title: "Skin Tone…", Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(p.skinToneView(e)) }})
	}
	return append(actions, search.Action{Title: "Copy Unicode", Label: "⌃⇧⌥C", Mods: mod | ui.Shift | ui.Alt, Key: ui.KeyC,
		Run: func(h search.Host) { p.copyText(h, e.Codepoints()) }})
}

// emojiKind names what a catalog entry is, for the row's kind.
func emojiKind(e emoji.Entry) string {
	if e.Kind() == emoji.KindEmoji {
		return "Emoji"
	}
	return "Symbol"
}

// emitEmoji copies or pastes a character, and remembers it as recent.
func (p *App) emitEmoji(h search.Host, e emoji.Entry, glyph string, paste bool) {
	p.rememberEmoji(e)
	if paste {
		p.pasteAfterHide(h, textEntry(glyph))
		return
	}
	p.copyText(h, glyph)
}
