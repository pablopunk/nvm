package design

import "github.com/egoist/mygo/ui"

// MenuRow is one action of an ActionMenu.
type MenuRow struct {
	Title string
	// Key is the action's shortcut, already spelled for the platform. Empty when it has none.
	Key string
	// Enter draws the return glyph in place of a key: the default action of the row.
	Enter    bool
	Selected bool
}

// MenuEvents is what the pointer did on an ActionMenu this frame: the row clicked and the row under the pointer, each
// -1 when none.
type MenuEvents struct {
	Clicked int
	Hovered int
}

// ActionMenu draws the action menu of a row (Ctrl+K) as a popover over the pane. Each action is a row; the selected one
// sits on the selection plate, a row under the pointer takes the wash of a list row, and the shortcut or return glyph sits
// at the end. It reports the row the pointer clicked or rests on, so the palette can run or choose it.
func ActionMenu(c *ui.Context, rows []MenuRow) MenuEvents {
	k := Tokens(c)
	events := MenuEvents{Clicked: -1, Hovered: -1}
	Popover(c, ActionPlacement, func() {
		for i, r := range rows {
			rowEl := ui.Row(c).Padding(6, 10).Radius(8).AlignItems(ui.Center).Gap(8)
			hovered := rowEl.Hovered()
			switch {
			case r.Selected:
				SelectedFill(rowEl, k, false)
			case hovered:
				rowEl.Background(k.Wash)
			}
			if hovered {
				events.Hovered = i
			}
			rowEl.Children(func() {
				ui.Text(c, r.Title).FontSize(13).TextColor(k.Strong).Grow(1).SingleLine()
				switch {
				case r.Key != "":
					ui.Text(c, r.Key).FontSize(11).FontWeight(500).TextColor(k.Muted).SingleLine()
				case r.Enter:
					ui.Icon(c, Icon("corner-down-left")).FontSize(12).TextColor(k.Muted)
				}
			})
			if rowEl.Clicked() {
				events.Clicked = i
			}
		}
	})
	return events
}
