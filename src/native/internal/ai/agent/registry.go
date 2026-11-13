package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// RunFunc runs one tool call. args is the JSON object the model sent, already checked against the tool's schema. It
// must return promptly once ctx is cancelled or times out.
type RunFunc func(ctx context.Context, args json.RawMessage) (Result, error)

// Result is what a tool returns. Text is what the model reads, and Outcome is the one line the transcript shows.
type Result struct {
	Text    string
	Outcome string
	// Failed ends the call as failed while the model still reads Text. A tool uses it for a failure the model can act on,
	// such as a program that did not compile.
	Failed bool
}

// Tool is one capability the model may call. Its name is what the model sees, so it names an intent, not a mechanism.
type Tool struct {
	Name        string
	Description string
	// Label is the words the transcript shows while the tool runs, such as "Searching files".
	Label string
	// Schema is the JSON Schema of the arguments: an object with properties, required names and additionalProperties.
	Schema json.RawMessage
	// Timeout is the wall-clock limit of one run. Zero selects the turn's tool timeout; a negative value sets none, for a
	// tool that bounds its own active time and asks the user inside the call, where waiting must not count.
	Timeout time.Duration
	// Confirm makes every call wait for the user's allow before it runs.
	Confirm bool
	// PathArgs names the arguments that hold a file path. A path policy checks them before the call runs.
	PathArgs []string
	// Summarize returns the one-line summary of a call's arguments shown in the transcript. Nil shows nothing.
	Summarize func(args json.RawMessage) string
	Run       RunFunc
}

// Registry is the set of tools a turn may offer and run. Names are unique and keep their registration order.
type Registry struct {
	tools map[string]Tool
	order []string
}

// NewRegistry returns a registry holding tools, or an error when a name is empty or registered twice.
func NewRegistry(tools ...Tool) (*Registry, error) {
	registry := &Registry{tools: make(map[string]Tool, len(tools))}
	for _, tool := range tools {
		if tool.Name == "" || tool.Run == nil {
			return nil, fmt.Errorf("agent: a tool needs a name and a run function")
		}
		if _, exists := registry.tools[tool.Name]; exists {
			return nil, fmt.Errorf("agent: tool %q is registered twice", tool.Name)
		}
		registry.tools[tool.Name] = tool
		registry.order = append(registry.order, tool.Name)
	}
	return registry, nil
}

// Lookup returns the tool registered under name.
func (r *Registry) Lookup(name string) (Tool, bool) {
	if r == nil {
		return Tool{}, false
	}
	tool, ok := r.tools[name]
	return tool, ok
}

// Wire returns the tools array of a chat request, in registration order, or nil when there are none.
func (r *Registry) Wire() (json.RawMessage, error) {
	if r == nil || len(r.order) == 0 {
		return nil, nil
	}
	type function struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		Parameters  json.RawMessage `json:"parameters"`
	}
	type wireTool struct {
		Type     string   `json:"type"`
		Function function `json:"function"`
	}
	wire := make([]wireTool, 0, len(r.order))
	for _, name := range r.order {
		tool := r.tools[name]
		wire = append(wire, wireTool{Type: "function", Function: function{Name: name, Description: tool.Description, Parameters: tool.Schema}})
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("agent: encoding the tools: %w", err)
	}
	return data, nil
}

// All returns the tools in registration order.
func (r *Registry) All() []Tool {
	if r == nil {
		return nil
	}
	all := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		all = append(all, r.tools[name])
	}
	return all
}
