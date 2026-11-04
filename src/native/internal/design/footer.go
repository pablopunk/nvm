package design

import "github.com/egoist/mygo/ui"

// KeyHint is an action named with the key that runs it, as the footer shows it.
type KeyHint struct {
	Title string
	// Key is the key's label, already spelled for the platform.
	Key string
}

// FooterData is the status bar under the palette: where the user is, and what Enter and the action menu do.
type FooterData struct {
	Name string
	// Primary is what Enter does and Secondary what the action menu key opens. Both are nil when the
	// selected row has no actions, in which case Hint, if set, is shown instead.
	Primary   *KeyHint
	Secondary *KeyHint
	Hint      string
}

// Footer draws the status bar of a view along the bottom edge of the pane.
func Footer(c *ui.Context, d FooterData) {
	k := Tokens(c)
	ui.Row(c).Padding(8, 14).Gap(14).AlignItems(ui.Center).Background(k.Wash).Children(func() {
		ui.Icon(c, Icon("zap")).FontSize(14).TextColor(k.Muted)
		ui.Text(c, d.Name).FontSize(12).TextColor(k.Muted).Grow(1).SingleLine()
		switch {
		case d.Primary != nil:
			keyHint(c, d.Primary)
			if d.Secondary != nil {
				keyHint(c, d.Secondary)
			}
		case d.Hint != "":
			ui.Text(c, d.Hint).FontSize(12).TextColor(k.Muted)
		}
	})
}

// keyHint draws an action's title with its key cap at the end of the footer.
func keyHint(c *ui.Context, hint *KeyHint) {
	k := Tokens(c)
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, hint.Title).FontSize(12).FontWeight(500).TextColor(k.Strong)
		Keycap(c, hint.Key)
	})
}
