package design

import "github.com/egoist/mygo/ui"

// ChipData is a small raised label: an optional icon, an optional bold word, then an optional text.
type ChipData struct {
	Icon string
	Bold string
	Text string
}

// Chip draws a hint such as "Tab to ask AI": a lens pill with the label in the muted ink.
func Chip(c *ui.Context, d ChipData) {
	k := Tokens(c)
	chip := ui.Row(c).Gap(6).AlignItems(ui.Center).Height(InnerHeight).Padding(0, 10).Radius(InnerRadius)
	Lens(chip, k).Children(func() {
		if d.Icon != "" {
			ui.Icon(c, Icon(d.Icon)).FontSize(12).TextColor(k.Muted)
		}
		if d.Bold != "" {
			ui.Text(c, d.Bold).FontSize(12).FontWeight(600).TextColor(k.Muted)
		}
		if d.Text != "" {
			ui.Text(c, d.Text).FontSize(12).TextColor(k.Muted)
		}
	})
}
