package app

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/search"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
)

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

func newID(seed string) string {
	sum := sha1.Sum([]byte(fmt.Sprint(seed, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:6])
}

// ---- snippets ----

type Snippet struct{ ID, Name, Keyword, Text string }

type SnippetStore struct{ Items []Snippet }

func newSnippetStore() *SnippetStore {
	s := &SnippetStore{}
	loadJSON("snippets.json", s)
	return s
}

func (s *SnippetStore) save() { saveJSON("snippets.json", s) }

func (p *App) snippetForm(existing *Snippet) *palette.View {
	sn := Snippet{}
	title := "Create Snippet"
	if existing != nil {
		sn, title = *existing, "Edit Snippet"
	}
	fields := []*FormField{
		{Label: "Name", Value: sn.Name, Placeholder: "Email signature"},
		{Label: "Keyword (optional, type it in the palette to find the snippet)", Value: sn.Keyword, Placeholder: ";sig"},
		{Label: "Content", Value: sn.Text, Multiline: true, Placeholder: "Text to paste…"},
	}
	return p.formView(title, fields, func(v []string) string {
		if strings.TrimSpace(v[0]) == "" || v[2] == "" {
			return "Name and content are required."
		}
		sn.Name, sn.Keyword, sn.Text = strings.TrimSpace(v[0]), strings.TrimSpace(v[1]), v[2]
		if existing != nil {
			for i := range p.snips.Items {
				if p.snips.Items[i].ID == sn.ID {
					p.snips.Items[i] = sn
				}
			}
		} else {
			sn.ID = newID(sn.Name)
			p.snips.Items = append(p.snips.Items, sn)
		}
		p.snips.save()
		return ""
	})
}

func (p *App) snippetItem(sn Snippet) search.Item {
	sub := oneLine(sn.Text)
	if sn.Keyword != "" {
		sub = sn.Keyword + "  ·  " + sub
	}
	return search.Item{
		ID: sn.ID, Title: sn.Name, Subtitle: sub, Icon: "type", Tone: "teal",
		Actions: []search.Action{
			{Title: "Paste", Run: func(h search.Host) { h.Hide(); p.pasteText(h, sn.Text) }},
			{Title: "Copy to Clipboard", Label: "⌃C", Mods: mod, Key: ui.KeyC, Run: func(h search.Host) { mygo.Clipboard.WriteText(sn.Text); h.Hide() }},
			{Title: "Edit", Label: "⌃E", Mods: mod, Key: ui.KeyE, Run: func(h search.Host) { h.Push(p.snippetForm(&sn)) }},
			{Title: "Delete", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Dismiss: search.DismissNever, Run: func(h search.Host) {
				p.snips.Items = slices.DeleteFunc(p.snips.Items, func(x Snippet) bool { return x.ID == sn.ID })
				p.snips.save()
				h.Refresh()
			}},
		},
		Detail: func(c *ui.Context, h search.Host) {
			ui.Scroll(c).Grow(1).Children(func() { ui.Text(c, sn.Text).FontSize(12) })
		},
	}
}

func (p *App) snippetsView() *palette.View {
	return &palette.View{
		Title: "Snippets", Placeholder: "Search Snippets", EmptyTitle: "No snippets yet.", EmptySub: "Create one to paste text quickly.",
		Search: func(q string) []search.Item {
			lq := strings.ToLower(q)
			out := []search.Item{{Title: "Create Snippet", Icon: "plus", Actions: []search.Action{{Title: "Create", Run: func(h search.Host) { h.Push(p.snippetForm(nil)) }}}}}
			for _, sn := range p.snips.Items {
				if lq == "" || strings.Contains(strings.ToLower(sn.Name+" "+sn.Keyword+" "+sn.Text), lq) {
					out = append(out, p.snippetItem(sn))
				}
			}
			return out
		},
	}
}

// ---- notes ----

type Note struct {
	ID      string
	Text    string
	Updated time.Time
}

type NoteStore struct {
	Items []Note
}

func newNoteStore() *NoteStore {
	s := &NoteStore{}
	loadJSON("notes.json", s)
	return s
}

func (s *NoteStore) save() { saveJSON("notes.json", s) }

func noteTitle(n Note) string {
	l := strings.TrimSpace(strings.SplitN(strings.TrimSpace(n.Text), "\n", 2)[0])
	if l == "" {
		return "Untitled note"
	}
	return oneLine(l)
}

func (p *App) noteEditor(id string) *palette.View {
	idx := func() int { return slices.IndexFunc(p.notes.Items, func(n Note) bool { return n.ID == id }) }
	return &palette.View{
		Title: "Note", HideInput: true, Hint: "esc Back   autosaves",
		OnLeave: func() { p.notes.save() },
		Status:  func() string { return "Saved" },
		Body: func(c *ui.Context, h search.Host) {
			i := idx()
			if i < 0 {
				return
			}
			ta := ui.TextArea(c, &p.notes.Items[i].Text).Grow(1).Margin(12).Padding(10).FontSize(15).Placeholder("Start typing…").AutoFocus()
			if ta.Changed() {
				p.notes.Items[i].Updated = time.Now()
				p.notes.save()
			}
		},
	}
}

func (p *App) newNote() {
	n := Note{ID: newID("note"), Updated: time.Now()}
	p.notes.Items = append([]Note{n}, p.notes.Items...)
	p.notes.save()
	p.Push(p.noteEditor(n.ID))
}

func (p *App) floatNote(id string) {
	i := slices.IndexFunc(p.notes.Items, func(n Note) bool { return n.ID == id })
	if i < 0 {
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title: "Note", Width: 340, Height: 300, AlwaysOnTop: true,
		Content: ui.View(func(c *ui.Context) {
			j := slices.IndexFunc(p.notes.Items, func(n Note) bool { return n.ID == id })
			if j < 0 {
				return
			}
			if ui.TextArea(c, &p.notes.Items[j].Text).Fill().Padding(12).FontSize(14).Changed() {
				p.notes.Items[j].Updated = time.Now()
				p.notes.save()
			}
		}),
	})
	w.Show()
}

func (p *App) notesView() *palette.View {
	return &palette.View{
		Title: "Quick Notes", Placeholder: "Search Quick Notes",
		Search: func(q string) []search.Item {
			lq := strings.ToLower(q)
			out := []search.Item{{Title: "New Note", Icon: "plus", Actions: []search.Action{{Title: "Create", Run: func(h search.Host) { p.newNote() }}}}}
			for _, n := range p.notes.Items {
				if lq != "" && !strings.Contains(strings.ToLower(n.Text), lq) {
					continue
				}
				n := n
				out = append(out, search.Item{
					ID: n.ID, Title: noteTitle(n), Subtitle: "Edited " + ago(n.Updated), Icon: "sticky-note", Tone: "yellow",
					Actions: []search.Action{
						{Title: "Open", Run: func(h search.Host) { h.Push(p.noteEditor(n.ID)) }},
						{Title: "Open as Floating Window", Label: "⌃F", Mods: mod, Key: ui.KeyF, Run: func(h search.Host) { h.Hide(); p.floatNote(n.ID) }},
						{Title: "Copy Text", Label: "⌃C", Mods: mod, Key: ui.KeyC, Run: func(h search.Host) { mygo.Clipboard.WriteText(n.Text); h.Hide() }},
						{Title: "Delete", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Dismiss: search.DismissNever, Run: func(h search.Host) {
							p.notes.Items = slices.DeleteFunc(p.notes.Items, func(x Note) bool { return x.ID == n.ID })
							p.notes.save()
							h.Refresh()
						}},
					},
					Detail: func(c *ui.Context, h search.Host) {
						ui.Scroll(c).Grow(1).Children(func() { ui.Text(c, n.Text).FontSize(12) })
					},
				})
			}
			return out
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
