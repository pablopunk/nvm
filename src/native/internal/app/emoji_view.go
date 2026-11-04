package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/emoji"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// The picker's grid: eight cells a row, as the 1.x picker lays them out. The grid shows the first 160 matches, as 1.x's
// MAX_VISIBLE_ITEMS does, so a broad query lists the same characters in both apps.
const (
	emojiColumns    = 8
	emojiCellHeight = 46
	emojiRowLimit   = 160
)

// emojiCategoryNames is the order Tab cycles the categories in: all characters first, then the catalog's groups.
var emojiCategoryNames = append([]string{emoji.AllCategory}, emojiCatalog.Categories()...)

// emojiPicker is the state of one Emoji & Symbols view: the category the grid shows.
type emojiPicker struct {
	category int // index into emojiCategoryNames
}

// current returns the category the grid searches, emoji.AllCategory for every entry.
func (picker *emojiPicker) current() string {
	return emojiCategoryNames[picker.category]
}

// categoryName is the category as the footer names it.
func (picker *emojiPicker) categoryName() string {
	if name := picker.current(); name != emoji.AllCategory {
		return name
	}
	return "All Characters"
}

// step moves to the next category (d = 1) or the previous one (d = -1), wrapping around.
func (picker *emojiPicker) step(d int) {
	n := len(emojiCategoryNames)
	picker.category = (picker.category + d + n) % n
}

// footer is the footer's left text: the category, or the chosen character's name, codepoints and category.
func (picker *emojiPicker) footer(chosen *search.Item) string {
	if chosen == nil {
		return picker.categoryName()
	}
	return chosen.Title + "  " + chosen.Subtitle + "  ·  " + picker.categoryName()
}

// onKey moves between categories with Tab and Shift+Tab. The search refreshes, so the grid shows the new category.
func (picker *emojiPicker) onKey(c *ui.Context, h search.Host) bool {
	switch {
	case c.Shortcut(0, ui.KeyTab):
		picker.step(1)
	case c.Shortcut(ui.Shift, ui.KeyTab):
		picker.step(-1)
	default:
		return false
	}
	h.Refresh()
	return true
}

// emojiView is the Emoji & Symbols picker: a grid of the catalog that searches as you type, copies or pastes the
// chosen character, and offers Copy Unicode and the skin tone of characters that have variants.
func (p *App) emojiView() *palette.View {
	picker := &emojiPicker{}
	return &palette.View{
		Title:       "Emoji & Symbols",
		Placeholder: "Search emoji, symbols, or Unicode values",
		Hint:        p.emojiHint(),
		EmptyTitle:  "No matching characters",
		EmptySub:    "Try a name, keyword, character, or Unicode value.",
		Limit:       emojiRowLimit,
		Providers: func() []search.Provider {
			return p.emojiProviders(picker.current(), p.skinTone())
		},
		Footer: picker.footer,
		OnKey:  picker.onKey,
		Grid: &palette.Grid{
			Columns:    emojiColumns,
			CellHeight: emojiCellHeight,
			Cell:       drawEmojiCell,
		},
	}
}

// emojiHint names the keys of the picker: Enter pastes where the platform can, and copies otherwise.
func (p *App) emojiHint() string {
	if p.canPaste() {
		return "↵ Paste   ⌃↵ Copy   ⇥ Category"
	}
	return "↵ Copy   ⇥ Category"
}

// drawEmojiCell draws the character of a cell. The palette draws the plate, the selection and the label.
func drawEmojiCell(c *ui.Context, it search.Item) {
	ui.Text(c, it.Glyph).FontSize(26)
}
