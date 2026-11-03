package design

import "github.com/egoist/mygo/ui"

// RowData is one list row. Every provider's row has this shape, so row height and rhythm stay the same.
type RowData struct {
	// Section, when set, is the header drawn above this row; callers set it only on the first row of a run.
	Section   string
	Title     string
	Subtitle  string
	Accessory string
	Tile      TileData
	// Tone colors the tile, and the title when TintTitle or Prominent is set.
	Tone      string
	TintTitle bool
	// Prominent is the accent-tinted row (Automate); Selected is the row under the cursor of the list.
	Prominent bool
	Selected  bool
	// ShowEnter draws the return glyph at the end of a selected row that has actions.
	ShowEnter bool
}

// Row draws one list row: tile, title over subtitle, an accessory at the end. A selected row sits on the warm
// selection plate with a faint gold edge; a prominent row carries the accent tint, stronger when selected.
func Row(c *ui.Context, d RowData) {
	k := Tokens(c)
	ui.Column(c).Children(func() {
		if d.Section != "" {
			Section(c, d.Section)
		}
		title, subtitle := k.Strong, k.Tertiary
		if d.TintTitle || d.Prominent {
			if col, ok := k.Tones[d.Tone]; ok {
				title = col
			}
		}
		rowEl := ui.Row(c).Padding(5, 8).Gap(10).AlignItems(ui.Center).Radius(12)
		if d.Prominent && !d.Selected {
			rowEl.Background(k.AccentSoft)
		}
		if d.Selected {
			SelectedFill(rowEl, k, d.Prominent)
		}
		tile := d.Tile
		tile.Title, tile.Tone, tile.Prominent = d.Title, d.Tone, d.Prominent
		rowEl.Children(func() {
			Tile(c, tile)
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, d.Title).FontSize(13).FontWeight(600).TextColor(title).SingleLine()
				if d.Subtitle != "" {
					ui.Text(c, d.Subtitle).FontSize(11).TextColor(subtitle).SingleLine()
				}
			})
			if d.Accessory != "" {
				ui.Text(c, d.Accessory).FontSize(12).TextColor(k.Muted).SingleLine()
			}
			if d.ShowEnter {
				ui.Icon(c, Icon("corner-down-left")).FontSize(13).TextColor(k.Muted)
			}
		})
	})
}
