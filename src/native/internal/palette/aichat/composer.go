package aichat

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

// composerRow is the composer under the transcript: a card that holds the palette's text input, with the tier picker
// and the send or stop control under the text, as the desktop app's composer is.
func composerRow(c *ui.Context, p *page, record chat.Chat, h search.Host) {
	k := design.Tokens(c)
	ui.Column(c).Radius(8).Background(k.Surface).Border(1, k.EdgeStrong).InsetShadow(0, 1, 0, 0, k.Hi).Children(func() {
		ui.Column(c).Children(func() {
			if len(p.attachments) > 0 {
				attachmentsRow(c, p)
			}
		})
		ui.TextAreaBase(c, &p.draft).Bind(&p.input).Lines(1, 6).Placeholder(placeholderFor(p.deps.Service, p.id)).
			PlaceholderColor(k.Faint).FontSize(13).TextColor(k.Strong).Padding(10, 12, 4, 12).
			OnPaste(p.paste).OnTextKey(func(key ui.TextKey) bool { return p.composerKey(key, h) })
		ui.Row(c).Padding(0, 6, 6, 6).AlignItems(ui.Center).Gap(4).Children(func() {
			tierPicker(c, p, record, h)
			ui.Row(c).Children(func() {
				if record.Tier == chat.TierSmart && p.deps.PickImages != nil {
					attachButton(c, p)
				}
			})
			ui.Spacer(c)
			sendOrStop(c, p, h)
		})
	})
}

// menuRunner is the palette's port for running the action its open menu chose. The composer's text takes Enter before
// the palette's keys, so Enter reaches the menu through this.
type menuRunner interface {
	RunChosenAction() bool
}

// composerKey answers the keys of the composer's text: Enter runs the chosen action of an open menu, otherwise it sends
// what is typed and clears it; Shift+Enter breaks the line, as the desktop app's composer does.
func (p *page) composerKey(key ui.TextKey, h search.Host) bool {
	if key.Key != ui.KeyEnter || key.Mods != 0 {
		return false
	}
	if menu, ok := h.(menuRunner); ok && menu.RunChosenAction() {
		return true
	}
	if p.send(key.Text(), h) {
		key.SetText("")
	}
	return true
}

// sendOrStop is the control at the end of the composer: Stop while a reply streams, otherwise send.
func sendOrStop(c *ui.Context, p *page, h search.Host) {
	k := design.Tokens(c)
	if p.deps.Service.Busy(p.id) {
		stop := ui.ButtonBase(c).Padding(4, 10).Radius(6).Background(k.Interactive)
		stop.Children(func() {
			ui.Text(c, "Stop").FontSize(12).FontWeight(600).TextColor(k.Danger)
		})
		if stop.Clicked() {
			p.deps.Service.Stop(p.id)
		}
		return
	}
	empty := strings.TrimSpace(p.draft) == "" && len(p.attachments) == 0
	send := ui.ButtonBase(c).Padding(4).Radius(14).Background(k.AccentFill).Disabled(empty)
	if empty {
		send.Opacity(0.35)
	}
	send.Children(func() {
		ui.Icon(c, design.Icon("corner-down-left")).FontSize(14).TextColor(k.AccentContrast)
	})
	if send.Clicked() && p.send(p.draft, h) {
		p.draft = ""
	}
}

// send sends text and the pictures attached to it as the next message, or answers a waiting tool call when there is
// neither. It reports whether the text was taken, so the composer clears it; a refused send keeps the text and the
// pictures, and the palette says why.
func (p *page) send(text string, h search.Host) bool {
	text = strings.TrimSpace(text)
	if text == "" && len(p.attachments) == 0 {
		if p.deps.Service.Awaiting(p.id) {
			report(h, p.deps.Service.Decide(p.id, true))
		}
		return false
	}
	err := p.deps.Service.SendWithImages(p.id, text, p.attachments)
	report(h, err)
	if err != nil {
		return false
	}
	p.attachments = nil
	return true
}

// tierPicker names the tier the next reply uses, and switches it on a click. Like the desktop app's select, it is
// disabled while a reply streams. The backend's model and route names are never shown.
func tierPicker(c *ui.Context, p *page, record chat.Chat, h search.Host) {
	k := design.Tokens(c)
	busy := p.deps.Service.Busy(p.id)
	picker := ui.ButtonBase(c).Padding(3, 8).Radius(6).Disabled(busy)
	picker.Children(func() {
		color := k.Secondary
		if busy {
			color = k.Faint
		}
		ui.Text(c, tierLabel(record.Tier)+" ⌄").FontSize(12).FontWeight(600).TextColor(color)
	})
	if picker.Clicked() {
		p.toggleTier(h)
	}
}

// tierLabel is the word the picker shows for a tier.
func tierLabel(tier chat.Tier) string {
	if tier == chat.TierSmart {
		return "Smart"
	}
	return "Fast"
}
