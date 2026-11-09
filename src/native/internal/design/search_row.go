package design

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// SearchData is the field at the top of the palette: an icon, then either the view's title or a text input.
type SearchData struct {
	// Loading animates only while the host is waiting on a user action or the view's data source.
	Loading bool
	// Title is shown in place of the input when HideInput is set.
	Title     string
	HideInput bool
	// Back makes the icon a button that goes back a level.
	Back bool
	// Placeholder, Query and Input are the input's hint, text and focus handle.
	Placeholder string
	Query       *string
	Input       *ui.Handle
	// AIHint shows the "Tab to ask AI" chip beside the input.
	AIHint bool
	// Concealed masks the text while a secret is typed; the query still holds the real text.
	Concealed bool
	// CaretAtEnd puts the caret after the last rune of Query in this frame, for a text the app just set.
	CaretAtEnd bool
	// OnKey sees the keys the input would take before it takes them, and reports whether it took one. Nil leaves
	// every key to the input.
	OnKey func(k ui.TextKey) bool
}

// SearchEvents reports what the user did in the search row this frame. The caller acts on them.
type SearchEvents struct {
	Back, Changed, Submitted bool
	// Selection is the start and end, in characters, of the input's selection as this frame built it; start <= end.
	Selection [2]int
}

// SearchRow draws the search field and reports its events. It draws no state of its own: the query and the
// focus live with the caller.
func SearchRow(c *ui.Context, d SearchData) SearchEvents {
	var events SearchEvents
	ui.Row(c).Padding(14, 18).Gap(12).AlignItems(ui.Center).Children(func() {
		k := Tokens(c)
		bolt := ui.Icon(c, Icon("zap")).FontSize(21).TextColor(k.Accent)
		events.Back = d.Back && bolt.Clicked()
		if d.Loading {
			ui.Spinner(c)
		}
		if d.HideInput {
			ui.Text(c, d.Title).FontSize(21).TextColor(k.Strong).Grow(1).SingleLine()
			return
		}
		in := ui.TextInputBase(c.Key("search-input"), d.Query).Bind(d.Input).Grow(1).FontSize(21).TextColor(k.Strong).
			Placeholder(d.Placeholder).PlaceholderColor(k.Faint).Label("Search")
		if d.Concealed {
			in = in.Password()
		}
		if d.CaretAtEnd {
			end := utf8.RuneCountInString(*d.Query)
			in = in.SetTextSelection(end, end)
		}
		if d.OnKey != nil {
			in = in.OnTextKey(d.OnKey)
		}
		anchor, caret := in.TextAnchorCaret()
		events.Selection = [2]int{min(anchor, caret), max(anchor, caret)}
		events.Changed = in.Changed()
		events.Submitted = in.Submitted()
		if d.AIHint {
			Chip(c, ChipData{Icon: "sparkles", Bold: "Tab", Text: "to ask AI"})
		}
	})
	return events
}
