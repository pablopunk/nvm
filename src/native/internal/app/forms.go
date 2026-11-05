package app

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ---- generic form view ----

type FormField struct {
	Label       string
	Value       string
	Placeholder string
	Multiline   bool
}

// formFooterHintOn is the form's footer on goos: the submit key is the palette's modifier with Enter, and Escape cancels.
func formFooterHintOn(goos string) string {
	return palette.KeyLabelOn("⌃↵", goos) + " Save   esc Cancel"
}

// formView is a palette sub-view with stacked fields; the palette's modifier with Enter submits. onSubmit returns an
// error text or "".
func (p *App) formView(title string, fields []*FormField, onSubmit func(vals []string) string) *palette.View {
	errText := ""
	submit := func() {
		vals := make([]string, len(fields))
		for i, f := range fields {
			vals[i] = f.Value
		}
		if errText = onSubmit(vals); errText == "" {
			p.Pop()
		}
	}
	return &palette.View{
		Title: title, HideInput: true, Hint: formFooterHintOn(runtime.GOOS),
		OnKey: func(c *ui.Context, h search.Host) bool {
			if c.Shortcut(mod, ui.KeyEnter) {
				submit()
				return true
			}
			return false
		},
		Body: func(c *ui.Context, h search.Host) {
			ui.Column(c).Padding(18).Gap(10).Grow(1).Children(func() {
				for i, f := range fields {
					design.FormField(c, design.FieldData{Label: f.Label, Placeholder: f.Placeholder, Value: &f.Value, Multiline: f.Multiline, Focus: i == 0})
				}
				if errText != "" {
					design.FormError(c, errText)
				}
			})
		},
	}
}
