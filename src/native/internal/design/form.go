package design

import "github.com/egoist/mygo/ui"

// FieldData is one input of a form: its label, its text and the hint of an empty one. Focus gives the field
// the keyboard focus when the form opens; a form focuses its first field.
type FieldData struct {
	Label       string
	Placeholder string
	Value       *string
	Multiline   bool
	Focus       bool
}

// FormField draws a labelled input: a bold label over a single-line field, or over a multi-line editor that
// takes the space left in its column.
func FormField(c *ui.Context, d FieldData) {
	k := Tokens(c)
	ui.Text(c, d.Label).FontSize(12).Bold().TextColor(k.Secondary)
	if d.Multiline {
		editor := ui.TextArea(c, d.Value).Grow(1).Placeholder(d.Placeholder).FontSize(13)
		if d.Focus {
			editor.AutoFocus()
		}
		return
	}
	field := ui.TextInput(c, d.Value).Placeholder(d.Placeholder)
	if d.Focus {
		field.AutoFocus()
	}
}

// FormError shows why a form did not save, under its fields.
func FormError(c *ui.Context, text string) {
	k := Tokens(c)
	ui.Text(c, text).TextColor(k.Danger).FontSize(12)
}
