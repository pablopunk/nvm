package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// previewWidth is the width of the preview beside a builder chat's transcript.
const previewWidth = 300

// PreviewRow is one row an installed extension shows in the preview beside its builder chat.
type PreviewRow struct {
	Title    string
	Subtitle string
}

// previewable reports whether the chat's preview can open: a builder chat bound to an extension, when the deps can read
// that extension's rows.
func (p *page) previewable(record chat.Chat) bool {
	return record.Kind == chat.KindBuilder && record.Extension != "" && p.deps.PreviewRows != nil
}

// previewing reports whether the preview is open for the chat.
func (p *page) previewing(record chat.Chat) bool {
	return p.showPreview && p.previewable(record)
}

// splitWithPreview draws the chat beside the preview of its extension: the chat takes the room the preview leaves.
func splitWithPreview(c *ui.Context, p *page, record chat.Chat, chatSide func()) {
	ui.Row(c).Grow(1).MinHeight(0).Gap(12).Children(func() {
		ui.Column(c).Grow(1).MinHeight(0).Children(chatSide)
		previewPane(c, record.Extension, p.deps.PreviewRows(record.Extension))
	})
}

// previewPane draws the rows an extension shows now, under its name. The rows are read on every frame from the
// snapshot the app keeps, so they show the extension as it is after each edit.
func previewPane(c *ui.Context, extension string, rows []PreviewRow) {
	k := design.Tokens(c)
	ui.Column(c).Width(previewWidth).MinHeight(0).Gap(6).Padding(8, 12).Radius(8).Background(k.Surface).Border(1, k.EdgeSubtle).Children(func() {
		ui.Text(c, "Preview · "+extension).FontSize(11).FontWeight(600).TextColor(k.Muted).NoWrap()
		for _, row := range rows {
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, row.Title).FontSize(12).FontWeight(600).TextColor(k.Strong).MaxLines(1)
				if row.Subtitle != "" {
					ui.Text(c, row.Subtitle).FontSize(11).TextColor(k.Muted).MaxLines(1)
				}
			})
		}
	})
}

// previewAction opens or closes the preview beside a builder chat, as 1.x's Focus Preview opens its builder workspace.
func (p *page) previewAction(record chat.Chat) search.Action {
	title := "Show Preview"
	if p.showPreview {
		title = "Hide Preview"
	}
	return search.Action{Title: title, Label: "⌃⇧P", Key: ui.KeyP, Mods: palette.Mod | ui.Shift, Dismiss: search.DismissNever,
		Run: func(search.Host) { p.showPreview = !p.showPreview }}
}
