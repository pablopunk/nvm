package services

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// AIToolHost lists and runs the AI tools enabled extensions declare, for the tools service. A tool of a disabled
// extension is unknown. A refusal, such as an unknown tool or arguments that do not fit its input, wraps
// api.ErrInvalidInput and runs nothing.
type AIToolHost interface {
	List(query string) []api.AIToolRecord
	// Plan finds the tool, checks args against its input and has the extension describe the call, without running it.
	Plan(ctx context.Context, extensionID, tool string, args json.RawMessage) (protocol.ToolPlan, error)
	// Call runs the tool. approved marks a call the user allowed, so the tool's own service calls do not ask again.
	Call(ctx context.Context, extensionID, tool string, args json.RawMessage, approved bool) (json.RawMessage, error)
}

// serveTools lists the AI tools of enabled extensions with the tools declaration and, from a palette action or a
// conversation program, calls one. A call whose class asks the user waits for the confirmation port, which is the
// chat's gate for a program; the extension cannot answer it.
func (h *Handler) serveTools(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if h.deps.Tools == nil {
		return nil, unavailable("extension AI tools are not available in this build")
	}
	if err := h.require(api.CapabilityTools); err != nil {
		return nil, err
	}
	switch call.Op {
	case protocol.OpList:
		var args protocol.QueryArgs
		if err := decodeArgs(call.Args, &args); err != nil {
			return nil, err
		}
		return h.deps.Tools.List(args.Query), nil
	case protocol.OpCall:
		return h.callTool(ctx, call)
	}
	return nil, invalid("tools has no operation %q", call.Op)
}

func (h *Handler) callTool(ctx context.Context, call protocol.ServiceCall) (json.RawMessage, error) {
	if err := requireRun(protocol.RequestMethodOf(ctx), "calling an extension's AI tool"); err != nil {
		return nil, err
	}
	var args protocol.ToolCallArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return nil, err
	}
	switch {
	case args.ExtensionID == "" || args.Tool == "":
		return nil, invalid("a tool call needs the extension id and the tool name")
	case args.ExtensionID == h.manifest.ID:
		return nil, invalid("an extension runs its own tools directly, not through the tools service")
	}
	plan, err := h.deps.Tools.Plan(ctx, args.ExtensionID, args.Tool, args.Args)
	if err != nil {
		return nil, err
	}
	approved := aiToolAsksEachTime(plan.Tool.Class)
	if approved || plan.Tool.Class == api.AIToolSensitiveRead {
		if err := h.confirmDestructive(ctx, plan.Question()); err != nil {
			return nil, err
		}
	}
	return h.deps.Tools.Call(ctx, args.ExtensionID, args.Tool, plan.Args, approved)
}

// aiToolAsksEachTime reports whether every call of the class needs the user's allow. Only such a call runs with the
// user's approval, so a tool asked about once for a read never removes data unasked.
func aiToolAsksEachTime(class api.AIToolClass) bool {
	return class == api.AIToolDestructive || class == api.AIToolExternal || class == api.AIToolSetting
}
