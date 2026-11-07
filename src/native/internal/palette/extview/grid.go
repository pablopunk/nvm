package extview

import (
	"github.com/egoist/mygo/ui"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// compactCellHeight is one glyph tile's height in a compact grid, which draws the glyph alone, as the emoji picker does.
const compactCellHeight = 48

// gridSpecOf is the palette grid of the view: compact layouts draw glyph tiles, every other layout draws titled tiles.
func (s *Session) gridSpecOf() *palette.Grid {
	if s.view.Grid != nil && s.view.Grid.Layout == "compact" {
		return &palette.Grid{Columns: s.gridColumns(), Compact: true, CellHeight: compactCellHeight, Cell: drawCompactCell}
	}
	if s.view.Grid != nil && s.view.Grid.Layout == "wide" {
		ratio := gridAspectRatio(s.view.Grid.AspectRatio)
		return &palette.Grid{Columns: s.gridColumns(), CellHeight: 180, ImageAspectRatio: ratio, Cell: func(c *ui.Context, it search.Item) { drawWideGridCell(c, it, ratio) }}
	}
	return &palette.Grid{Columns: s.gridColumns(), CellHeight: gridCellHeight, Cell: drawGridCell}
}

// drawCompactCell draws the glyph of a tile alone, or its icon when it has no glyph.
func drawCompactCell(c *ui.Context, it search.Item) {
	if it.Glyph != "" {
		ui.Text(c, it.Glyph).FontSize(26)
		return
	}
	design.Tile(c, design.TileData{Image: it.Image, Icon: it.Icon, Tone: it.Tone, Title: it.Title})
}

// selectionFooter is the left footer of a grid with SelectionFooter: the chosen tile's title and subtitle, or nothing.
func selectionFooter(chosen *search.Item) string {
	if chosen == nil {
		return ""
	}
	return chosen.Title + "  " + chosen.Subtitle
}

// gridCellHeight is one tile's height: the tile, a title line and the cell padding.
const gridCellHeight = 104

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
	k := design.Tokens(c)
	ui.Column(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
		if it.Glyph != "" {
			ui.Text(c, it.Glyph).FontSize(26)
		} else {
			design.Tile(c, design.TileData{Image: it.Image, Icon: it.Icon, Tone: it.Tone, Title: it.Title})
		}
		ui.Text(c, it.Title).FontSize(13).TextColor(k.Strong).FillWidth().SingleLine()
		if it.Subtitle != "" {
			ui.Text(c, it.Subtitle).FontSize(11).TextColor(k.Secondary).FillWidth().SingleLine()
		}
	})
}

// drawWideGridCell gives media images the space to be recognized at a glance.
func drawWideGridCell(c *ui.Context, it search.Item, ratio float32) {
	k := design.Tokens(c)
	ui.Column(c).FillWidth().MinWidth(0).Gap(6).Children(func() {
		if it.Image != nil {
			ui.Image(c, it.Image).FillWidth().AspectRatio(ratio).Fit(ui.Cover).Radius(8).Clip()
		} else {
			ui.Row(c).FillWidth().AspectRatio(ratio).Center().Children(func() { design.Tile(c, design.TileData{Icon: it.Icon, Tone: it.Tone, Title: it.Title}) })
		}
		ui.Text(c, it.Title).FontSize(13).TextColor(k.Strong).FillWidth().SingleLine()
		if it.Subtitle != "" {
			ui.Text(c, it.Subtitle).FontSize(11).TextColor(k.Secondary).FillWidth().SingleLine()
		}
	})
}

func gridAspectRatio(value string) float32 {
	parts := strings.Split(value, "/")
	if len(parts) == 2 {
		width, errWidth := strconv.ParseFloat(strings.TrimSpace(parts[0]), 32)
		height, errHeight := strconv.ParseFloat(strings.TrimSpace(parts[1]), 32)
		if errWidth == nil && errHeight == nil && width > 0 && height > 0 && width/height >= 0.25 && width/height <= 4 {
			return float32(width / height)
		}
	}
	return 16.0 / 9
}
