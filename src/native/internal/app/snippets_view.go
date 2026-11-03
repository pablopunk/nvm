package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/snippets"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// snippetsView lists the snippets, filtered by name, keyword or text, with a row that creates one.
func (p *App) snippetsView() *palette.View {
	return &palette.View{
		Title: "Snippets", Placeholder: "Search Snippets", EmptyTitle: "No snippets yet.", EmptySub: "Create one to paste text quickly.",
		Search: func(q string) []search.Item {
			out := []search.Item{{Title: "Create Snippet", Icon: "plus", Actions: []search.Action{{Title: "Create", Run: func(h search.Host) { h.Push(p.snippetForm(nil)) }}}}}
			for _, sn := range p.snippetMatches(q) {
				out = append(out, p.snippetItem(sn))
			}
			return out
		},
	}
}

// snippetMatches returns the snippets whose name, keyword or text contains the query.
func (p *App) snippetMatches(q string) []snippets.Snippet {
	if p.snips == nil {
		return nil
	}
	all := p.snips.List()
	needle := strings.ToLower(strings.TrimSpace(q))
	if needle == "" {
		return all
	}
	var out []snippets.Snippet
	for _, sn := range all {
		if strings.Contains(strings.ToLower(sn.Name+" "+sn.Keyword+" "+sn.Text), needle) {
			out = append(out, sn)
		}
	}
	return out
}

// snippetItem is one snippet: its name, keyword and first line, and the actions. The first action runs on Enter:
// Paste when the platform can send the paste gesture, Copy when it cannot. Usage is learned under the snippet.
func (p *App) snippetItem(sn snippets.Snippet) search.Item {
	sub := oneLine(sn.Text)
	if sn.Keyword != "" {
		sub = sn.Keyword + "  ·  " + sub
	}
	paste := search.Action{Title: "Paste", Run: func(h search.Host) { h.Hide(); p.pasteSnippet(sn) }}
	copyAction := search.Action{Title: "Copy to Clipboard", Label: "⌃C", Mods: mod, Key: ui.KeyC, Run: func(h search.Host) { p.copySnippet(sn); h.Hide() }}
	it := search.Item{
		ID: sn.ID, Title: sn.Name, Subtitle: sub, Icon: "type", Tone: "teal", LearnKey: "snippet:" + sn.ID,
		Detail: func(c *ui.Context, _ search.Host) { snippetDetail(c, sn) },
	}
	if p.canPaste() {
		it.Actions = []search.Action{paste, copyAction}
	} else {
		it.Actions = []search.Action{copyAction}
	}
	it.Actions = append(it.Actions,
		search.Action{Title: "Edit", Label: "⌃E", Mods: mod, Key: ui.KeyE, Run: func(h search.Host) { h.Push(p.snippetForm(&sn)) }},
		search.Action{Title: "Delete", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Dismiss: search.DismissNever, Run: func(h search.Host) {
			h.Push(palette.Confirm(fmt.Sprintf("Delete “%s”", sn.Name), "This cannot be undone.", func() { p.deleteSnippet(sn.ID) }))
		}},
	)
	return it
}

// snippetDetail draws the text of a snippet and a line on the placeholders it can use.
func snippetDetail(c *ui.Context, sn snippets.Snippet) {
	t := design.Tokens(c)
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Text(c, sn.Text).FontSize(13).TextColor(t.Primary)
	})
	ui.Divider(c)
	ui.Text(c, "Placeholders: {clipboard} and {date}").FontSize(11).TextColor(t.Tertiary)
}

// canPaste reports whether the platform can send the paste gesture. Where it cannot, the snippet is copied.
func (p *App) canPaste() bool {
	return p.ports.Capabilities.Supported(platform.CapabilityClipboardPaste)
}

// pasteSnippet expands the snippet, puts it on the clipboard and sends the paste gesture. When the gesture is
// refused the text stays on the clipboard for a manual paste.
func (p *App) pasteSnippet(sn snippets.Snippet) {
	_, _ = snippets.Paste(sn, clock(), p.ports.Clipboard, p.ports.Paster, "")
}

// copySnippet puts the expanded snippet on the clipboard.
func (p *App) copySnippet(sn snippets.Snippet) {
	_ = snippets.Copy(sn, clock(), p.ports.Clipboard)
}

// deleteSnippet removes a snippet after the user confirmed it.
func (p *App) deleteSnippet(id string) {
	if err := p.snips.Delete(id); err != nil {
		p.Toast(snippetErrorMessage(err))
		return
	}
	p.Toast("Snippet deleted")
}

// snippetForm is the form that creates a snippet, or edits the given one. Name and content are required; the
// keyword is optional and unique.
func (p *App) snippetForm(existing *snippets.Snippet) *palette.View {
	title, in := "Create Snippet", snippets.Input{}
	if existing != nil {
		title, in = "Edit Snippet", snippets.Input{Name: existing.Name, Keyword: existing.Keyword, Text: existing.Text}
	}
	fields := []*FormField{
		{Label: "Name", Value: in.Name, Placeholder: "Email signature"},
		{Label: "Keyword (optional, type it in the palette to find the snippet)", Value: in.Keyword, Placeholder: ";sig"},
		{Label: "Content", Value: in.Text, Multiline: true, Placeholder: "Text to paste…"},
	}
	return p.formView(title, fields, func(v []string) string {
		if p.snips == nil {
			return "Snippets could not be opened"
		}
		form := snippets.Input{Name: v[0], Keyword: v[1], Text: v[2]}
		var err error
		if existing != nil {
			_, err = p.snips.Update(existing.ID, form)
		} else {
			_, err = p.snips.Create(form)
		}
		if err != nil {
			return snippetFormMessage(err)
		}
		return ""
	})
}

// snippetFormMessage is the line the form shows under its fields for a refused save.
func snippetFormMessage(err error) string {
	var taken *snippets.KeywordTakenError
	switch {
	case errors.As(err, &taken):
		return fmt.Sprintf("Keyword “%s” is already used by “%s”. Choose another keyword.", taken.Keyword, taken.Name)
	case errors.Is(err, snippets.ErrNameRequired):
		return "Name is required."
	case errors.Is(err, snippets.ErrTextRequired):
		return "Content is required."
	case errors.Is(err, snippets.ErrKeywordHasSpace):
		return "A keyword is one word, with no spaces."
	case errors.Is(err, snippets.ErrTooLong):
		return "That is too long for a snippet."
	}
	return "Could not save the snippet: " + err.Error()
}

// snippetErrorMessage says why a snippet could not be changed.
func snippetErrorMessage(err error) string {
	if errors.Is(err, snippets.ErrNotFound) {
		return "This snippet no longer exists"
	}
	return "Could not change the snippet: " + err.Error()
}

// snippetRootRows answers a root query with the snippets whose keyword is the query (exact, ranked first) or starts
// with it. Snippets without a keyword are found by the snippets view only.
func snippetRootRows(p *App, q string) []search.Item {
	if p.snips == nil {
		return nil
	}
	exact, prefix := p.snips.Match(q)
	var out []search.Item
	for _, sn := range exact {
		out = append(out, p.snippetRootRow(sn, scoreSnippetExact))
	}
	for _, sn := range prefix {
		out = append(out, p.snippetRootRow(sn, scoreSnippet))
	}
	return out
}

// snippetRootRow is a snippet's row at the root, with its score and the source label.
func (p *App) snippetRootRow(sn snippets.Snippet, score int) search.Item {
	it := p.snippetItem(sn)
	it.Kind = search.KindSnippets
	it.Score = score
	return it
}
