package app

import (
	"log"

	"github.com/pablopunk/nvm/native/internal/features/emoji"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

const (
	// emojiRecentLimit is how many recent characters lead the picker.
	emojiRecentLimit = 24
	// emojiRecentKeep is how many recent characters are remembered.
	emojiRecentKeep = 64
)

// emojiRecents is emoji-recents.json: the ids of the characters copied or pasted, newest first.
type emojiRecents struct {
	IDs []string
}

// recentIDSource exposes the remembered characters to the catalog (emoji.RecentSource).
type recentIDSource struct {
	app *App
}

// RecentIDs returns the remembered ids, newest first. Without a store there are none.
func (s recentIDSource) RecentIDs() []string {
	if s.app.recents == nil {
		return nil
	}
	v, _ := s.app.recents.Load()
	return v.IDs
}

// rememberEmoji moves a character to the front of the recents, once.
func (p *App) rememberEmoji(e emoji.Entry) {
	if p.recents == nil {
		return
	}
	id := e.ID()
	err := p.recents.Update(func(v *emojiRecents) { v.IDs = prependUniqueID(v.IDs, id, emojiRecentKeep) })
	if err != nil {
		log.Println("emoji recents:", err)
	}
}

// prependUniqueID returns ids with id first, the other ids in their order, at most keep of them.
func prependUniqueID(ids []string, id string, keep int) []string {
	out := make([]string, 0, min(len(ids)+1, keep))
	out = append(out, id)
	for _, other := range ids {
		if other != id && len(out) < keep {
			out = append(out, other)
		}
	}
	return out
}

// skinTone is the skin tone the picker applies to every character that has variants. A stored value that names no
// tone reads as the default.
func (p *App) skinTone() emoji.SkinTone {
	tone := emoji.SkinTone(p.S.EmojiSkinTone)
	if tone > emoji.SkinToneDark {
		return emoji.SkinToneDefault
	}
	return tone
}

// setSkinTone remembers the skin tone for every character.
func (p *App) setSkinTone(tone emoji.SkinTone) {
	p.S.EmojiSkinTone = int(tone)
	p.S.save()
}

// skinToneView lists the six tones of one character. Choosing one remembers it and returns to the picker.
func (p *App) skinToneView(e emoji.Entry) *palette.View {
	return &palette.View{
		Title:       "Skin Tone",
		Placeholder: "Choose a skin tone",
		Search: func(string) []search.Item {
			current := p.skinTone()
			items := make([]search.Item, 0, int(emoji.SkinToneDark)+1)
			for tone := emoji.SkinToneDefault; tone <= emoji.SkinToneDark; tone++ {
				items = append(items, skinToneItem(e, tone, tone == current, p))
			}
			return items
		},
	}
}

// skinToneItem is the row of one tone. Its action remembers the tone, goes back to the picker and searches again,
// so the grid shows the new glyphs.
func skinToneItem(e emoji.Entry, tone emoji.SkinTone, current bool, p *App) search.Item {
	subtitle := ""
	if current {
		subtitle = "Current"
	}
	return search.Item{
		Kind:     "Skin tone",
		Title:    tone.Label(),
		Subtitle: subtitle,
		Glyph:    e.WithSkinTone(tone),
		Actions: []search.Action{{Title: "Use " + tone.Label(), Dismiss: search.DismissNever, Run: func(h search.Host) {
			p.setSkinTone(tone)
			h.Pop()
			h.Refresh()
		}}},
	}
}
