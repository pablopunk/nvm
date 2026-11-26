package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// AIToolSource is what the AI tool host reads and runs. *Manager implements it.
type AIToolSource interface {
	DeclaredAITools() []api.AIToolRecord
	DescribeAITool(ctx context.Context, id string, call api.AIToolCall) (string, error)
	CallAITool(ctx context.Context, id string, call api.AIToolCall) (json.RawMessage, error)
}

var errAIToolsUnavailable = fmt.Errorf("extension AI tools cannot be used before the extensions start: %w", api.ErrNotAvailable)

// AIToolHost lists and runs the AI tools of enabled extensions, as services.AIToolHost says. It never runs a tool whose
// arguments do not fit its input, and it looks the tool up again before each call, so a tool of an extension turned off
// meanwhile does not run.
type AIToolHost struct {
	source func() AIToolSource
}

// NewAIToolHost returns the host over source, which returns nil until the extensions start.
func NewAIToolHost(source func() AIToolSource) AIToolHost {
	return AIToolHost{source: source}
}

// List returns the tools that match every word of query.
func (h AIToolHost) List(query string) []api.AIToolRecord {
	source := h.source()
	if source == nil {
		return nil
	}
	words := strings.Fields(strings.ToLower(query))
	var out []api.AIToolRecord
	for _, tool := range source.DeclaredAITools() {
		text := strings.ToLower(strings.Join([]string{tool.ExtensionID, tool.ExtensionTitle, tool.Name, tool.Description}, " "))
		if holdsEveryWord(text, words) {
			out = append(out, tool)
		}
	}
	return out
}

// Plan checks the call and has the extension describe it.
func (h AIToolHost) Plan(ctx context.Context, extensionID, tool string, args json.RawMessage) (protocol.ToolPlan, error) {
	source, record, args, err := h.checkedCall(extensionID, tool, args)
	if err != nil {
		return protocol.ToolPlan{Tool: record, Args: args}, err
	}
	description, err := source.DescribeAITool(ctx, extensionID, api.AIToolCall{Name: tool, Args: args})
	return protocol.ToolPlan{Tool: record, Args: args, Description: strings.TrimSpace(description)}, err
}

// Call checks the call again and runs it, with the user's approval on ctx when approved.
func (h AIToolHost) Call(ctx context.Context, extensionID, tool string, args json.RawMessage, approved bool) (json.RawMessage, error) {
	source, _, args, err := h.checkedCall(extensionID, tool, args)
	if err != nil {
		return nil, err
	}
	if approved {
		ctx = protocol.WithUserApproval(ctx)
	}
	return source.CallAITool(ctx, extensionID, api.AIToolCall{Name: tool, Args: args})
}

// checkedCall finds the tool among the enabled extensions' tools and checks args against its input.
func (h AIToolHost) checkedCall(extensionID, tool string, args json.RawMessage) (AIToolSource, api.AIToolRecord, json.RawMessage, error) {
	source := h.source()
	if source == nil {
		return nil, api.AIToolRecord{}, args, errAIToolsUnavailable
	}
	record, found := findAITool(source.DeclaredAITools(), extensionID, tool)
	if !found {
		return nil, record, args, fmt.Errorf("no enabled extension %q has the tool %q; list the tools first: %w", extensionID, tool, api.ErrInvalidInput)
	}
	args, err := checkAIToolArgs(record.InputSchema, args)
	return source, record, args, err
}

func findAITool(tools []api.AIToolRecord, extensionID, name string) (api.AIToolRecord, bool) {
	for _, tool := range tools {
		if tool.ExtensionID == extensionID && tool.Name == name {
			return tool, true
		}
	}
	return api.AIToolRecord{ExtensionID: extensionID, Name: name}, false
}
