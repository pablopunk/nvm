package design

import "github.com/egoist/mygo/ui"

// SelectData is a popup button: the title of the selected option, then up and down chevrons.
type SelectData struct {
	// Options are the titles, in order. Selected is the index of the current one, checked in the menu.
	Options  []string
	Selected int
	// Disabled grays the control out and opens no menu, as while a reply streams.
	Disabled bool
}

// Select draws a popup button. A press opens the platform's menu below it, as a select's does: the options in order,
// a check mark on the selected one. It returns the index of the option chosen in that menu this frame, or -1 when none
// was. Escape and a click elsewhere close the menu with no choice.
func Select(c *ui.Context, d SelectData) int {
	k := Tokens(c)
	color := k.Secondary
	if d.Disabled {
		color = k.Faint
	}
	chosen := -1
	button := ui.ButtonBase(c).Padding(3, 8).Radius(6).Disabled(d.Disabled)
	button.Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Text(c, selectedTitle(d)).FontSize(12).FontWeight(600).TextColor(color).SingleLine()
			ui.Icon(c, Icon("chevrons-up-down")).FontSize(10).TextColor(color)
		})
	})
	button.Menu(func(m *ui.Menu) {
		chosen = fillSelectMenu(d.Options, d.Selected, func(title string, checked bool) bool {
			return m.Item(title).Checked(checked).Chosen()
		})
	})
	return chosen
}

// selectedTitle is the title of the selected option, or "" when no option has that index.
func selectedTitle(d SelectData) string {
	if d.Selected < 0 || d.Selected >= len(d.Options) {
		return ""
	}
	return d.Options[d.Selected]
}

// fillSelectMenu adds one item per option through item, checking the selected one. It returns the index whose item
// reports the choice, or -1.
func fillSelectMenu(options []string, selected int, item func(title string, checked bool) bool) int {
	chosen := -1
	for i, title := range options {
		if item(title, i == selected) {
			chosen = i
		}
	}
	return chosen
}
