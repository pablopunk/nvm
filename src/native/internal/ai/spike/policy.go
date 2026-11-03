package spike

import "fmt"

// Decision is the policy verdict for one tool call. Reason is shown to the model when the call is refused.
type Decision struct {
	Allow  bool
	Reason string
}

// Policy decides whether a tool call may run and may redact what a tool returned. Decide runs before the tool; a
// refused call never executes. FilterResult runs after the tool and before the output is capped.
type Policy interface {
	Decide(call ToolCall, tool Tool) Decision
	FilterResult(call ToolCall, tool Tool, output string) string
}

// allowAll is the policy used when the caller passes none.
type allowAll struct{}

func (allowAll) Decide(ToolCall, Tool) Decision {
	return Decision{Allow: true}
}

func (allowAll) FilterResult(_ ToolCall, _ Tool, output string) string {
	return output
}

// NameAllowlist allows only the tools named in it. It is the by-name half of the 1.x tool policy: pi exposes exactly
// the names the host lists and refuses the rest.
type NameAllowlist map[string]bool

// AllowNames returns a NameAllowlist for names.
func AllowNames(names ...string) NameAllowlist {
	allowlist := make(NameAllowlist, len(names))
	for _, name := range names {
		allowlist[name] = true
	}
	return allowlist
}

// Decide allows calls whose tool name is listed and refuses every other name.
func (allowlist NameAllowlist) Decide(call ToolCall, _ Tool) Decision {
	if allowlist[call.Name] {
		return Decision{Allow: true}
	}
	return Decision{Reason: fmt.Sprintf("%s is not an allowed tool", call.Name)}
}

// FilterResult passes output through unchanged.
func (NameAllowlist) FilterResult(_ ToolCall, _ Tool, output string) string {
	return output
}

// ChainPolicy applies policies in order. The first refusal wins, and every FilterResult runs in order.
type ChainPolicy []Policy

// Decide returns the first refusal, or an allow when every policy allows.
func (chain ChainPolicy) Decide(call ToolCall, tool Tool) Decision {
	for _, policy := range chain {
		if decision := policy.Decide(call, tool); !decision.Allow {
			return decision
		}
	}
	return Decision{Allow: true}
}

// FilterResult threads output through every policy.
func (chain ChainPolicy) FilterResult(call ToolCall, tool Tool, output string) string {
	for _, policy := range chain {
		output = policy.FilterResult(call, tool, output)
	}
	return output
}
