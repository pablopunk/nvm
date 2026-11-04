package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// clipRowActions are the actions a row offers after its copy: preview, and paste text where the entry holds text
// and the platform can paste. Paste is omitted without the capability rather than shown broken.
func (p *App) clipRowActions(e clipboard.Entry, row clipboard.Row) []search.Action {
	var actions []search.Action
	for _, a := range row.Actions {
		switch a.Type {
		case clipboard.ActionPreview:
			actions = append(actions, p.clipPreviewAction(e.ID, row.Title))
		case clipboard.ActionPaste:
			if p.clip.caps.Supported(platform.CapabilityClipboardPaste) {
				actions = append(actions, p.clipPasteTextAction(e.ID))
			}
		}
	}
	return actions
}

// clipPreviewAction opens the entry full-palette, as the Preview action of 1.x does.
func (p *App) clipPreviewAction(id, title string) search.Action {
	return search.Action{Title: "Preview", Label: "⌃Y", Mods: mod, Key: ui.KeyY, Run: func(h search.Host) {
		entry, ok := p.clip.history.Get(id)
		if !ok {
			h.Toast("That item is no longer in clipboard history.")
			return
		}
		h.Push(p.clipPreviewView(entry, title))
	}}
}

// clipPreviewView is the full-palette preview of one entry: the same content as the side pane, with the list area
// to itself.
func (p *App) clipPreviewView(e clipboard.Entry, title string) *palette.View {
	return &palette.View{
		Title:       title,
		Placeholder: title,
		HideInput:   true,
		Body: func(c *ui.Context, _ search.Host) {
			p.clipDetail(c, e)
		},
	}
}

// clipPasteTextAction pastes the entry as plain text into the app that had focus.
func (p *App) clipPasteTextAction(id string) search.Action {
	return search.Action{Title: "Paste Text", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Dismiss: search.DismissNever,
		Run: func(h search.Host) { p.runClipPasteText(h, id) }}
}

// runClipPasteText pastes the text of an entry, without its markup, into the app that had focus.
func (p *App) runClipPasteText(h search.Host, id string) {
	entry, ok := p.clip.history.Get(id)
	if !ok {
		h.Toast("That item is no longer in clipboard history.")
		return
	}
	p.pasteAfterHide(h, "Paste Text", plainTextOf(entry))
}

// plainTextOf is the entry as plain text under the same ID, so the paste is recorded against the history entry.
func plainTextOf(e clipboard.Entry) clipboard.Entry {
	return clipboard.Entry{ID: e.ID, Kind: clipboard.KindText, Text: e.Text}
}
