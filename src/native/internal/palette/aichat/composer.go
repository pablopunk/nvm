package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

// composerRow is the row under the transcript: the tier picker, and Stop while a reply is in flight.
func composerRow(c *ui.Context, p *page, record chat.Chat, h search.Host) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		tierPicker(c, p, record, h)
		if p.deps.Service.Busy(p.id) && ui.Button(c, "Stop").Clicked() {
			p.deps.Service.Stop(p.id)
		}
	})
}

// tierPicker names the tier the next reply uses, and switches it on a click. The backend's model and route names are
// never shown.
func tierPicker(c *ui.Context, p *page, record chat.Chat, h search.Host) {
	k := design.Tokens(c)
	picker := ui.ButtonBase(c).Padding(3, 8).Radius(6).Background(k.Surface).Border(1, k.EdgeStrong).InsetShadow(0, 1, 0, 0, k.Hi)
	picker.Children(func() {
		ui.Text(c, tierLabel(record.Tier)+" ⌄").FontSize(12).FontWeight(600).TextColor(k.Muted)
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
