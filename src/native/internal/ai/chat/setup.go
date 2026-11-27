package chat

import "github.com/pablopunk/nvm/native/internal/ai/agent"

// setupFor is the loop setup of a chat: its model tier and, for conversations, the tools.
func (s *Service) setupFor(chat Chat) (agent.Setup, *BuilderTurn) {
	var builderTurn *BuilderTurn
	setup := agent.Setup{
		Model: &routeModel{
			client:    s.opts.Client,
			tier:      chat.Tier,
			dispatch:  s.opts.Dispatch,
			note:      s.noteRoute,
			preflight: s.opts.Preflight,
		},
		Clock: s.opts.Clock,
	}
	switch {
	case chat.Kind == KindConversation:
		setup.Tools = s.opts.Tools.Registry
		setup.Policy = s.opts.Tools.Policy
		setup.Waiver = currentMessageWaiver(s.opts.Tools, chat)
	case chat.Kind == KindBuilder && s.opts.BuilderTools != nil:
		tools := s.opts.BuilderTools(chat.ID)
		builderTurn = tools.BuilderTurn
		setup.Tools = tools.Registry
		setup.Policy = tools.Policy
		setup.MaxSteps = BuilderMaxSteps
		setup.ToolTimeout = BuilderToolTimeout
	}
	return setup, builderTurn
}

// currentMessageWaiver is the toolset's waiver for the user's message this turn answers. Older messages grant nothing.
func currentMessageWaiver(tools Toolset, chat Chat) agent.Waiver {
	if tools.Waiver == nil {
		return nil
	}
	return tools.Waiver(currentUserText(chat))
}

// currentUserText is the text of the chat's last user message, the one the turn answers.
func currentUserText(chat Chat) string {
	for i := len(chat.Messages) - 1; i >= 0; i-- {
		if chat.Messages[i].Author == AuthorUser {
			return chat.Messages[i].Text
		}
	}
	return ""
}
