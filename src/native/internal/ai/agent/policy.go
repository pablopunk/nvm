package agent

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// Decision is a policy's verdict on one call. Reason is shown to the model when the call is refused.
type Decision struct {
	Allow  bool
	Reason string
}

// Policy decides whether a call may run at all. A refused call never reaches its tool. Confirmation is not a policy
// decision: a tool that asks for it always waits for the user.
type Policy interface {
	Decide(call backend.ToolCall, tool Tool) Decision
}

// AllowAll is the policy used when a turn sets none.
type AllowAll struct{}

// Decide allows every call.
func (AllowAll) Decide(backend.ToolCall, Tool) Decision {
	return Decision{Allow: true}
}

// ChainPolicy applies policies in order and returns the first refusal.
type ChainPolicy []Policy

// Decide returns the first refusal, or an allow when every policy allows.
func (chain ChainPolicy) Decide(call backend.ToolCall, tool Tool) Decision {
	for _, policy := range chain {
		if decision := policy.Decide(call, tool); !decision.Allow {
			return decision
		}
	}
	return Decision{Allow: true}
}

// Refuse returns the refusal a policy gives for reason.
func Refuse(format string, args ...any) Decision {
	return Decision{Reason: fmt.Sprintf(format, args...)}
}
