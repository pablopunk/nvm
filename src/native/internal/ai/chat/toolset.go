package chat

import "github.com/pablopunk/nvm/native/internal/ai/agent"

// Toolset is the tools a conversation may call and the policy each call passes before it runs.
type Toolset struct {
	Registry *agent.Registry
	Policy   agent.Policy
	// Waiver, when set, is the waiver of a turn whose current user message is message, such as the launch grant.
	Waiver func(message string) agent.Waiver
	// BuilderTurn binds builder callbacks to this toolset's turn, never a later turn of the same chat.
	BuilderTurn *BuilderTurn
}
