package design

import "github.com/egoist/mygo/ui"

// NoticeData is a banner between the search field and the results: what is wrong, and the one button that
// fixes it. Alert marks a problem (the title takes the danger color) rather than a plain status.
type NoticeData struct {
	Icon        string
	Title       string
	Detail      string
	ActionLabel string
	Alert       bool
}

// Notice draws the banner and reports whether its button was pressed this frame.
func Notice(c *ui.Context, d NoticeData) bool {
	k := Tokens(c)
	var pressed bool
	ui.Row(c).Padding(8, 14).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, Icon(d.Icon)).FontSize(14).TextColor(k.Muted)
		ui.Column(c).Grow(1).MinHeight(0).Children(func() {
			title := k.Strong
			if d.Alert {
				title = k.Danger
			}
			ui.Text(c, d.Title).FontSize(12).FontWeight(600).TextColor(title).SingleLine()
			ui.Text(c, d.Detail).FontSize(11).TextColor(k.Muted).SingleLine()
		})
		pressed = ui.Button(c, d.ActionLabel).Clicked()
	})
	return pressed
}
