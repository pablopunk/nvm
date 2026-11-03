package spike

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ToolFunc runs one tool. args is a JSON object. It must return promptly once ctx is cancelled or times out.
type ToolFunc func(ctx context.Context, args json.RawMessage) (string, error)

// Tool is a registered tool. A zero Timeout uses the agent default.
type Tool struct {
	Spec    ToolSpec
	Run     ToolFunc
	Timeout time.Duration
}

// Registry is the set of tools the loop may expose and run. Names are unique.
type Registry struct {
	tools map[string]Tool
	order []string
}

// NewRegistry returns a registry holding tools, or an error when two tools share a name.
func NewRegistry(tools ...Tool) (*Registry, error) {
	registry := &Registry{tools: make(map[string]Tool, len(tools))}
	for _, tool := range tools {
		name := tool.Spec.Name
		if _, exists := registry.tools[name]; exists {
			return nil, fmt.Errorf("tool %q is registered twice", name)
		}
		registry.tools[name] = tool
		registry.order = append(registry.order, name)
	}
	return registry, nil
}

// Lookup returns the tool registered under name.
func (r *Registry) Lookup(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// Specs returns the tool descriptions in registration order.
func (r *Registry) Specs() []ToolSpec {
	specs := make([]ToolSpec, 0, len(r.order))
	for _, name := range r.order {
		specs = append(specs, r.tools[name].Spec)
	}
	return specs
}
