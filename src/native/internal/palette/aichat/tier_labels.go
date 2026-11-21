package aichat

import "github.com/pablopunk/nvm/native/internal/ai/chat"

// tierOrder is the order the tier menu lists the tiers in, as the desktop app's select does.
var tierOrder = []chat.Tier{chat.TierFast, chat.TierSmart}

// tierTitles are the words the tier menu shows, in tierOrder.
func tierTitles() []string {
	titles := make([]string, 0, len(tierOrder))
	for _, tier := range tierOrder {
		titles = append(titles, tierLabel(tier))
	}
	return titles
}

// tierLabel is the word the picker shows for a tier.
func tierLabel(tier chat.Tier) string {
	if tier == chat.TierSmart {
		return "Smart"
	}
	return "Fast"
}
