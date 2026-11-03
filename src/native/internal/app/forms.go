package app

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// newID returns a short random-looking id for a chat or a form record: a hash of the seed and the time.
func newID(seed string) string {
	sum := sha1.Sum([]byte(fmt.Sprint(seed, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:6])
}

// ---- generic form view ----

type FormField struct {
	Label       string
	Value       string
	Placeholder string
	Multiline   bool
}

// formView is a palette sub-view with stacked fields; Ctrl+Enter submits. onSubmit returns an error text or "".
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
		Title: title, HideInput: true, Hint: "⌃↵ Save   esc Cancel",
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

// ---- quicklinks ----

func (p *App) quicklinkForm() *palette.View {
	fields := []*FormField{
		{Label: "Name", Placeholder: "Hacker News"},
		{Label: "Keyword", Placeholder: "hn"},
		{Label: "URL (use {query} for the search text)", Placeholder: "https://hn.algolia.com/?q={query}"},
	}
	return p.formView("Create Quicklink", fields, func(v []string) string {
		if strings.TrimSpace(v[0]) == "" || strings.TrimSpace(v[1]) == "" || !strings.HasPrefix(v[2], "http") {
			return "Name, keyword and an http(s) URL are required."
		}
		p.S.Quicklinks = append(p.S.Quicklinks, Quicklink{Name: strings.TrimSpace(v[0]), Keyword: strings.TrimSpace(v[1]), URL: strings.TrimSpace(v[2])})
		p.S.save()
		return ""
	})
}
