package app

import (
	"github.com/pablopunk/nvm/native/internal/features/emoji"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

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
