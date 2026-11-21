package emoji

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/emoji"
)

// tone is the skin tone the picker applies to every character that has variants. A stored value that names no tone
// reads as the default.
func (e *Extension) tone(ctx api.Context) emoji.SkinTone {
	var stored int
	found, err := ctx.Storage().Get(toneKey, &stored)
	if err != nil {
		return emoji.SkinToneDefault
	}
	if !found {
		return validTone(e.legacy)
	}
	return validTone(emoji.SkinTone(stored))
}

// validTone keeps a stored tone that names a tone, and reads anything else as the default.
func validTone(tone emoji.SkinTone) emoji.SkinTone {
	if tone > emoji.SkinToneDark {
		return emoji.SkinToneDefault
	}
	return tone
}

// skinView lists the six tones of one character. Choosing one remembers it, goes back to the picker and searches
// again, so the grid shows the new glyphs.
func (e *Extension) skinView(ctx api.Context, id, query string) api.View {
	idx := e.catalog(ctx)
	record, _ := idx.recordOf(id)
	current := e.tone(ctx)
	items := make([]api.Item, 0, int(emoji.SkinToneDark)+1)
	for tone := emoji.SkinToneDefault; tone <= emoji.SkinToneDark; tone++ {
		subtitle := ""
		if tone == current {
			subtitle = "Current"
		}
		items = append(items, api.Item{
			ID: fmt.Sprintf("tone-%d", int(tone)), Title: tone.Label(), Subtitle: subtitle,
			Glyph: emoji.ApplySkinTone(record.Fields["glyph"], tone), TileGlyph: true,
			Primary: api.Primary(api.Run(useToneAction, "Use "+tone.Label(), map[string]string{
				toneArgName: fmt.Sprint(int(tone)), queryArgName: query,
			})),
		})
	}
	return api.View{Kind: api.ViewList, Title: "Skin Tone", Placeholder: "Choose a skin tone", Items: items}
}

// useTone remembers the tone the user chose, searches the picker again and goes back to it.
func (e *Extension) useTone(ctx api.Context, args map[string]string) (api.Result, error) {
	if err := ctx.Storage().Set(toneKey, intArg(args, toneArgName)); err != nil {
		return api.Result{}, err
	}
	if query := args[queryArgName]; query != "" {
		ctx.Views().RefreshSearch(query)
	}
	return ctx.Navigation().Pop(), nil
}
