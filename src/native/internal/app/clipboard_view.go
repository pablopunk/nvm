package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// clock is the instant relative times are measured from. Golden tests replace it with a fixed one.
var clock = time.Now

// clipRowLimit caps the rows of the history view. The features package uses the same default.
const clipRowLimit = clipboard.DefaultLimit

// clipIconUnit is the Lucide icon of text-like entries; images and file lists have their own.
const clipIconUnit = "clipboard"

// ago describes how long ago t was, in the words the clipboard rows use.
func ago(t time.Time) string {
	d := clock().Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// oneLine collapses s to one line of at most 90 characters.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 90 {
		s = string(r[:90]) + "…"
	}
	return s
}

// clipboardView is the Clipboard History view: the entries that match the query, newest first. An empty result shows the 1.x empty state.
func (p *App) clipboardView() *palette.View {
	return &palette.View{
		Title: clipboard.ViewTitle, Placeholder: clipboard.ViewPlaceholder,
		EmptyTitle: clipboard.EmptyTitle, EmptySub: clipboard.EmptySubtitle,
		Status: func() string { return fmt.Sprintf("%d items", p.clip.history.Len()) },
		Search: func(q string) []search.Item { return p.clipItems(q, clipRowLimit) },
	}
}

// clipItems returns the rows of the history that match q.
func (p *App) clipItems(q string, limit int) []search.Item {
	historyLen := p.clip.history.Len()
	entries := p.clip.history.Find(clipboard.Query{Text: q, Limit: limit})
	out := make([]search.Item, 0, len(entries))
	for _, e := range entries {
		out = append(out, p.clipItem(e, historyLen))
	}
	return out
}

// clipItem is one history entry as a palette row: its 1.x title, a subtitle that says when it was copied and what
// it is, the copy action on Enter, and the detail pane. Actions run by entry ID and look the entry up when they
// run, so a row never acts on a stale copy.
func (p *App) clipItem(e clipboard.Entry, historyLen int) search.Item {
	row := clipboard.BuildRow(e, historyLen)
	it := search.Item{
		ID:        row.ID,
		Kind:      search.KindClipboard,
		Title:     row.Title,
		Subtitle:  "Copied " + ago(e.CreatedAt) + " · " + clipKindLabel(e.Kind),
		Icon:      clipIcon(e.Kind),
		Tone:      "blue",
		TintTitle: true,
		Actions:   p.clipActions(e, row, historyLen),
		Detail:    func(c *ui.Context, _ search.Host) { p.clipDetail(c, e) },
	}
	return it
}

// clipKindLabel names what an entry holds, for the subtitle.
func clipKindLabel(kind clipboard.Kind) string {
	switch kind {
	case clipboard.KindImage:
		return "Image"
	case clipboard.KindFiles:
		return "Files"
	}
	return "Text"
}

// clipIcon is the Lucide icon of an entry's kind.
func clipIcon(kind clipboard.Kind) string {
	switch kind {
	case clipboard.KindImage:
		return "image"
	case clipboard.KindFiles:
		return "file-text"
	}
	return clipIconUnit
}

// clipActions is the action list of a row, in the order the menu shows: copy (Enter), paste (when the platform
// can), pin or unpin, ask AI for text, the removal actions, and the history-wide clears. Paste is omitted where
// the capability is missing rather than shown broken.
func (p *App) clipActions(e clipboard.Entry, row clipboard.Row, historyLen int) []search.Action {
	id := e.ID
	actions := []search.Action{{Title: row.Primary.Title, Dismiss: search.DismissNever, Run: func(h search.Host) { p.runClipCopy(h, id) }}}
	actions = append(actions, search.Action{Title: "Preview", Label: "⌃Y", Mods: mod, Key: ui.KeyY, Run: func(h search.Host) {
		h.Push(p.clipPreviewView(e))
	}})
	if p.clip.caps.Supported(platform.CapabilityClipboardPaste) && isTextCopy(e) {
		actions = append(actions, search.Action{Title: "Paste Text", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Dismiss: search.DismissNever,
			Run: func(h search.Host) { p.runClipPaste(h, id) }})
	}
	for _, removal := range row.Manage {
		actions = append(actions, p.clipRemovalAction(removal))
	}
	return actions
}

// isTextCopy reports whether a copy is text, with or without markup. Only those have a Paste Text action in 1.x.
func isTextCopy(e clipboard.Entry) bool {
	return e.Kind == clipboard.KindText || e.Kind == clipboard.KindHTML || e.Kind == clipboard.KindRTF
}

// clipRemovalAction is a removal from the history. It needs a confirmation, which the palette shows as a view
// above the list; a submenu opens its own list of removals.
func (p *App) clipRemovalAction(a clipboard.Action) search.Action {
	if a.Type == clipboard.ActionSubmenu {
		submenu := a.Submenu
		return search.Action{Title: a.Title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			h.Push(p.clipSubmenuView(submenu))
		}}
	}
	action := search.Action{Title: a.Title, Dismiss: search.DismissNever, Run: func(h search.Host) {
		h.Push(palette.Confirm(a.Title, a.ConfirmMessage, func() {
			p.removeEntries(clipboard.RangeQuery{Range: a.Range, ItemID: a.ItemID})
		}))
	}}
	if a.Range == clipboard.RangeItem {
		action.Label, action.Mods, action.Key = "⌃⌫", mod, ui.KeyBackspace
	}
	return action
}

// clipPreviewView is the full-palette preview of one copy: the same picture or text as its detail pane.
func (p *App) clipPreviewView(e clipboard.Entry) *palette.View {
	return &palette.View{
		Title:       "Preview",
		Placeholder: "Preview",
		HideInput:   true,
		Body: func(c *ui.Context, _ search.Host) {
			if e.Kind == clipboard.KindImage {
				p.clipImageDetail(c, e)
				return
			}
			p.clipDetail(c, e)
		},
	}
}

// clipSubmenuView is the list a "Remove Last..." action opens: one confirmed removal per time window.
func (p *App) clipSubmenuView(submenu *clipboard.Submenu) *palette.View {
	return &palette.View{
		Title:       submenu.Title,
		Placeholder: submenu.Title,
		Search: func(string) []search.Item {
			items := make([]search.Item, 0, len(submenu.Actions))
			for _, a := range submenu.Actions {
				items = append(items, search.Item{Title: a.Title, Icon: clipIconUnit, Tone: "blue", Actions: []search.Action{p.clipRemovalAction(a)}})
			}
			return items
		},
	}
}

// removeEntries removes the entries a range selects and says how many went.
func (p *App) removeEntries(q clipboard.RangeQuery) {
	n, err := p.clip.history.Remove(q)
	if err != nil {
		p.Toast(clipChangeMessage(err))
		return
	}
	p.Toast(clipboard.RemovedMessage(n))
}

// runClipCopy copies an entry to the clipboard and closes the palette.
func (p *App) runClipCopy(h search.Host, id string) {
	entry, ok := p.clip.history.Get(id)
	if !ok {
		h.Toast("That item is no longer in clipboard history.")
		return
	}
	if err := p.clip.copy(entry); err != nil {
		h.Toast(copyFailureMessage(err))
		return
	}
	h.Hide()
}

// runClipPaste pastes an entry into the app that had focus.
func (p *App) runClipPaste(h search.Host, id string) {
	entry, ok := p.clip.history.Get(id)
	if !ok {
		h.Toast("That item is no longer in clipboard history.")
		return
	}
	p.pasteAfterHide(h, entry)
}

// clipChangeMessage says why a change to the history was refused.
func clipChangeMessage(err error) string {
	if err == clipboard.ErrReadOnly {
		return "Clipboard history is read-only: it was written by a newer version of Nevermind."
	}
	return "Clipboard history could not be changed."
}
