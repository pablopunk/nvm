package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// compactCellHeight is one glyph tile's height in a compact grid, which draws the glyph alone, as the emoji picker does.
const compactCellHeight = 46

// gridSpecOf is the palette grid of the view: compact layouts draw glyph tiles, every other layout draws titled tiles.
func (s *Session) gridSpecOf() *palette.Grid {
	if s.view.Grid != nil && s.view.Grid.Layout == "compact" {
		return &palette.Grid{Columns: s.gridColumns(), CellHeight: compactCellHeight, Cell: drawCompactCell}
	}
	return &palette.Grid{Columns: s.gridColumns(), CellHeight: gridCellHeight, Cell: drawGridCell}
}

// drawCompactCell draws the glyph of a tile alone, or its icon when it has no glyph.
func drawCompactCell(c *ui.Context, it search.Item) {
	if it.Glyph != "" {
		ui.Text(c, it.Glyph).FontSize(26)
		return
	}
	design.Tile(c, design.TileData{Icon: it.Icon, Tone: it.Tone, Title: it.Title})
}

// selectionFooter is the footer of a grid with SelectionFooter: the chosen tile's title and subtitle, then the view's
// subtitle, or the view's subtitle alone when no tile is chosen.
func (s *Session) selectionFooter(chosen *search.Item) string {
	category := s.categoryTitle()
	if chosen == nil {
		return category
	}
	text := chosen.Title + "  " + chosen.Subtitle
	if category != "" {
		text += "  ·  " + category
	}
	return text
}

// categoryTitle is the title of the search accessory's option that is shown, or the view's subtitle without one.
func (s *Session) categoryTitle() string {
	if s.accessory == nil || s.view.SearchAccessory == nil {
		return s.view.Subtitle
	}
	for _, option := range s.view.SearchAccessory.Options {
		if option.Value == s.accessory.Value {
			return option.Title
		}
	}
	return s.view.Subtitle
}

// gridCellHeight is one tile's height: the tile, a title line and the cell padding.
const gridCellHeight = 96

// defaultGridColumns is the column count of a grid that names none.
const defaultGridColumns = 4

// gridColumns is the view's column count, or the default when it names none or names a count the palette cannot fit.
func (s *Session) gridColumns() int {
	if s.view.Grid != nil && s.view.Grid.Columns > 0 {
		return s.view.Grid.Columns
	}
	return defaultGridColumns
}

// drawGridCell draws one tile: the glyph when the item has one, else its icon, then the title under it.
func drawGridCell(c *ui.Context, it search.Item) {
	ui.Column(c).Gap(6).Children(func() {
		if it.Glyph != "" {
			ui.Text(c, it.Glyph).FontSize(26)
		} else {
			design.Tile(c, design.TileData{Icon: it.Icon, Tone: it.Tone, Title: it.Title})
		}
		ui.Text(c, it.Title).FontSize(11)
	})
}
