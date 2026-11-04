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

// emojiItem is the row of one character. Copy is the first action, so Enter copies; Paste is Ctrl+Enter, and the
// action menu holds Copy Unicode and, for characters with variants, the skin tone.
func (p *App) emojiItem(e emoji.Entry, tone emoji.SkinTone, score int) search.Item {
	glyph := e.Glyph()
	if e.SupportsSkinTone() {
		glyph = e.WithSkinTone(tone)
	}
	actions := []search.Action{
		{Title: "Copy", Run: func(h search.Host) { p.emitEmoji(h, e, glyph, false) }},
		{Title: "Paste", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) { p.emitEmoji(h, e, glyph, true) }},
		{Title: "Copy Unicode", Run: func(h search.Host) { p.copyText(h, e.Codepoints()) }},
	}
	if e.SupportsSkinTone() {
		actions = append(actions, search.Action{Title: "Skin Tone…", Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(p.skinToneView(e)) }})
	}
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
		p.pasteAfterHide(h, pasteActionTitle, textEntry(glyph))
		return
	}
	p.copyText(h, glyph)
}
