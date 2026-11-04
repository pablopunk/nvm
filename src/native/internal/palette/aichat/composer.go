package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

// composerRow is the row under the transcript: the tier the next reply uses, and Stop while a reply is in flight.
func composerRow(c *ui.Context, p *page, record chat.Chat, h search.Host) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		modelPill(c, record)
		if p.deps.Service.Busy(p.id) && ui.Button(c, "Stop").Clicked() {
			p.deps.Service.Stop(p.id)
		}
	})
}

// modelPill names the tier the next reply uses. The backend's model and route names are never shown.
func modelPill(c *ui.Context, record chat.Chat) {
	design.Chip(c, design.ChipData{Icon: "sparkles", Bold: tierLabel(record.Tier)})
}

// tierLabel is the word the pill shows for a tier.
func tierLabel(tier chat.Tier) string {
	if tier == chat.TierSmart {
		return "Smart"
	}
	return "Fast"
}
