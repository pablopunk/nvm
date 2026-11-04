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
	// Disabled draws the action muted, with Subtitle beneath it as the reason it cannot run. Choosing it does nothing.
	Disabled bool
	Subtitle string
}

// ActionMenu draws the action menu of a row (Ctrl+K) as a popover over the pane. Each action is a row; the
// selected one sits on the selection plate; its shortcut or return glyph sits at the end.
func ActionMenu(c *ui.Context, rows []MenuRow) {
	k := Tokens(c)
	Popover(c, ActionPlacement, func() {
		for _, r := range rows {
			rowEl := PlatePadding(ui.Row(c).Radius(8).AlignItems(ui.Center).Gap(8), r.Selected, 6, 10)
			if r.Selected {
				SelectedFill(rowEl, k, false)
			}
			rowEl.Children(func() {
				if r.Subtitle != "" {
					disabledRowContent(c, k, r)
					return
				}
				ui.Text(c, r.Title).FontSize(13).TextColor(actionTitleColor(k, r)).Grow(1).SingleLine()
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

// disabledRowContent draws a disabled menu row as 1.x does: the title muted, and the reason beneath it.
func disabledRowContent(c *ui.Context, k *TokenSet, r MenuRow) {
	ui.Column(c).Grow(1).Gap(2).Children(func() {
		ui.Text(c, r.Title).FontSize(13).TextColor(actionTitleColor(k, r)).SingleLine()
		ui.Text(c, r.Subtitle).FontSize(11).TextColor(k.Muted).SingleLine()
	})
}

// actionTitleColor is the title color of a menu row: strong for an action that runs, muted for one that is disabled.
func actionTitleColor(k *TokenSet, r MenuRow) ui.Color {
	if r.Disabled {
		return k.Muted
	}
	return k.Strong
}
