package design

import "github.com/egoist/mygo/ui"

// RowData is one list row. Every provider's row has this shape, so row height and rhythm stay the same.
type RowData struct {
	// Section, when set, is the header drawn above this row; callers set it only on the first row of a run.
	Section   string
	Title     string
	Subtitle  string
	Accessory string
	// Key, when set, draws its key cap at the end of the row, in place of Accessory.
	Key  string
	Tile TileData
	// Tone colors the tile, and the title when TintTitle or Prominent is set.
	Tone      string
	TintTitle bool
	// Prominent is the accent-tinted row (Automate); Selected is the row under the cursor of the list.
	Prominent bool
	Selected  bool
	// Disabled draws the row muted with no plate; Contextual draws it as a wrapped guidance line.
	Disabled   bool
	Contextual bool
	// ShowEnter draws the return glyph at the end of a selected row that has actions.
	ShowEnter bool
}

// Row draws one list row: tile, title over subtitle, an accessory at the end. A selected row sits on the warm
// selection plate with a faint gold edge; a prominent row carries the accent tint, stronger when selected; a
// plain row under the pointer takes a faint wash, weaker than the selection plate.
func Row(c *ui.Context, d RowData) {
	k := Tokens(c)
	ui.Column(c).Children(func() {
		if d.Section != "" {
			Section(c, d.Section)
		}
		if d.Contextual {
			ui.Column(c).Gap(2).Padding(5, 8).Children(func() {
				ui.Text(c, d.Title).FontSize(13).FontWeight(600).TextColor(k.Strong)
				if d.Subtitle != "" {
					ui.Text(c, d.Subtitle).FontSize(11).TextColor(k.Secondary)
				}
			})
			return
		}
		title, subtitle := k.Strong, k.Tertiary
		if d.TintTitle || d.Prominent {
			if col, ok := k.Tones[d.Tone]; ok {
				title = col
			}
		}
		rowEl := PlatePadding(ui.Row(c).Gap(10).AlignItems(ui.Center).Radius(12), d.Selected, 5, 8)
		hovered := rowEl.Hovered()
		switch {
		case d.Disabled:
			title, subtitle = k.Faint, k.Faint
		case d.Selected:
			SelectedFill(rowEl, k, d.Prominent)
		case d.Prominent:
			rowEl.Background(k.AccentSoft)
		case hovered:
			rowEl.Background(k.Wash)
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
			if d.Key != "" {
				Keycap(c, d.Key)
			}
			if d.ShowEnter {
				ui.Icon(c, Icon("corner-down-left")).FontSize(13).TextColor(k.Muted)
			}
		})
	})
}
