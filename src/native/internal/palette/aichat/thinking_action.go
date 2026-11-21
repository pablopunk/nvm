package aichat

import (
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/search"
)

// thinkingAction is the Show or Hide Thinking row of the newest reply that has reasoning, or nil when none has.
func (x *ExtensionChat) thinkingAction() *search.Action {
	index := latestReasoning(x.spec.Messages)
	if index < 0 {
		return nil
	}
	action := x.page.thinkingToggleAction(index)
	return &action
}

// withThinkingRow places the reasoning row before the tier row, as the AI Chats menu lists it after Stop. Without a tier
// row it goes last.
func withThinkingRow(actions []search.Action, thinking *search.Action) []search.Action {
	if thinking == nil {
		return actions
	}
	at := len(actions)
	for i, action := range actions {
		if strings.HasPrefix(action.Title, tierRowPrefix) {
			at = i
			break
		}
	}
	return slices.Insert(actions, at, *thinking)
}

// tierRowPrefix starts the title of the tier row, such as Switch to Smart.
const tierRowPrefix = "Switch to "

// thinkingToggleAction opens or closes the reasoning block of entry index, for keyboard users and the menu.
func (p *page) thinkingToggleAction(index int) search.Action {
	title := "Show Thinking"
	if p.thinkingOpen[index] {
		title = "Hide Thinking"
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
		p.thinkingOpen[index] = !p.thinkingOpen[index]
		h.Refresh()
	}}
}
