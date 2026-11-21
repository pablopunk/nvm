package tools

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ai/guard"
	"github.com/pablopunk/nvm/native/internal/backend"
)

// pathPolicy refuses a call whose path argument the guard refuses. Only the arguments a tool lists as paths are
// checked; an argument that is missing is left to the tool, which then uses the home folder.
type pathPolicy struct {
	guard *guard.Guard
	home  string
}

// Decide checks every path argument of the call.
func (p pathPolicy) Decide(call backend.ToolCall, tool agent.Tool) agent.Decision {
	if len(tool.PathArgs) == 0 {
		return agent.Decision{Allow: true}
	}
	var arguments map[string]json.RawMessage
	if err := json.Unmarshal([]byte(normalizedArguments(call.Function.Arguments)), &arguments); err != nil {
		return agent.Decision{Reason: "the arguments could not be read"}
	}
	for _, name := range tool.PathArgs {
		raw, present := arguments[name]
		if !present {
			continue
		}
		var path string
		if err := json.Unmarshal(raw, &path); err != nil {
			return agent.Decision{Reason: "the path must be text"}
		}
		if decision := p.guard.Check(path, p.home); !decision.Allow {
			return agent.Decision{Reason: decision.Reason}
		}
	}
	return agent.Decision{Allow: true}
}

func normalizedArguments(arguments string) string {
	if arguments == "" {
		return "{}"
	}
	return arguments
}
