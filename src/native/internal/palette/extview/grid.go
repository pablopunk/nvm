package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

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
