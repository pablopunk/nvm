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

// ActionMenu draws the action menu of a row (Ctrl+K) as a popover over the pane. Each action is a row; the
// selected one sits on the selection plate; its shortcut or return glyph sits at the end.
func ActionMenu(c *ui.Context, rows []MenuRow) {
	k := Tokens(c)
	Popover(c, ActionPlacement, func() {
		for _, r := range rows {
			rowEl := ui.Row(c).Padding(6, 10).Radius(8).AlignItems(ui.Center).Gap(8)
			if r.Selected {
				SelectedFill(rowEl, k, false)
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
		}
	})
}
