package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// previewable reports whether a chat can show its extension's preview beside it: a builder chat bound to an extension,
// when the deps can offer that extension's side.
func (p *page) previewable(record chat.Chat) bool {
	return record.Kind == chat.KindBuilder && record.Extension != "" && p.deps.Preview != nil
}

// previewing reports whether the preview is open beside the chat. It opens once the chat is bound to its extension, as
// 1.x opens the preview after a write, and Hide Preview closes it.
func (p *page) previewing(record chat.Chat) bool {
	return p.previewable(record) && !p.previewHidden
}

// sideFor is the side the page draws beside its transcript now: the extension's preview, or nil when none is open.
func (p *page) sideFor() *palette.Side {
	record, ok := p.deps.Service.Chat(p.id)
	if !ok || !p.previewing(record) {
		return nil
	}
	return p.deps.Preview(record.Extension)
}

// previewAction shows or hides the preview beside a builder chat, as ⌃⇧P does in 1.x's builder workspace.
func (p *page) previewAction(record chat.Chat) search.Action {
	title := "Show Preview"
	if p.previewing(record) {
		title = "Hide Preview"
	}
	return search.Action{Title: title, Label: "⌃⇧P", Key: ui.KeyP, Mods: palette.Mod | ui.Shift, Dismiss: search.DismissNever,
		Run: func(h search.Host) {
			p.previewHidden = !p.previewHidden
			h.Refresh()
		}}
}

// focusPreviewAction gives the keyboard to the preview, as 1.x's Focus Preview does. Escape gives it back.
func (p *page) focusPreviewAction() search.Action {
	return search.Action{Title: "Focus Preview", Dismiss: search.DismissNever, Run: func(search.Host) {
		p.sideFor().Focus()
	}}
}
