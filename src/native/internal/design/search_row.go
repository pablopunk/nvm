package design

import "github.com/egoist/mygo/ui"

// SearchData is the field at the top of the palette: an icon, then either the view's title or a text input.
type SearchData struct {
	// Title is shown in place of the input when HideInput is set.
	Title     string
	HideInput bool
	// Back makes the icon a button that goes back a level.
	Back bool
	// Placeholder, Query and Input are the input's hint, text and focus handle.
	Placeholder string
	Query       *string
	Input       *ui.Handle
	// Accessory is the label of the picker at the end of the field; "" draws none.
	Accessory string
	// AIHint shows the "Tab to ask AI" chip beside the input.
	AIHint bool
}

// SearchEvents reports what the user did in the search row this frame. The caller acts on them.
type SearchEvents struct {
	Back, Changed, Submitted bool
	Accessory                bool
}

// SearchRow draws the search field and reports its events. It draws no state of its own: the query and the
// focus live with the caller.
func SearchRow(c *ui.Context, d SearchData) SearchEvents {
	var events SearchEvents
	ui.Row(c).Padding(14, 18).Gap(12).AlignItems(ui.Center).Children(func() {
		k := Tokens(c)
		bolt := ui.Icon(c, Icon("zap")).FontSize(21).TextColor(k.Accent)
		events.Back = d.Back && bolt.Clicked()
		if d.HideInput {
			ui.Text(c, d.Title).FontSize(21).TextColor(k.Strong).Grow(1).SingleLine()
			return
		}
		in := ui.TextInputBase(c, d.Query).Bind(d.Input).Grow(1).FontSize(21).TextColor(k.Strong).
			Placeholder(d.Placeholder).PlaceholderColor(k.Faint).Label("Search")
		events.Changed = in.Changed()
		events.Submitted = in.Submitted()
		if d.Accessory != "" {
			events.Accessory = ui.Text(c, d.Accessory+" ⌄").FontSize(12).TextColor(k.Secondary).SingleLine().Clicked()
		}
		if d.AIHint {
			Chip(c, ChipData{Icon: "sparkles", Bold: "Tab", Text: "to ask AI"})
		}
	})
	return events
}
