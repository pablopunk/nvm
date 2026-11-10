package chat

import "github.com/pablopunk/nvm/native/internal/ai/agent"

// setupFor is the loop setup of a chat: its model tier and, for conversations, the tools.
func (s *Service) setupFor(chat Chat) agent.Setup {
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
	case chat.Kind == KindBuilder && s.opts.BuilderTools != nil:
		tools := s.opts.BuilderTools(chat.ID)
		setup.Tools = tools.Registry
		setup.Policy = tools.Policy
		setup.MaxSteps = BuilderMaxSteps
		setup.ToolTimeout = BuilderToolTimeout
	}
	return setup
}
