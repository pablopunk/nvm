package protocol

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// The tools service lists and calls the AI tools of enabled extensions.
const (
	ServiceTools = "tools"
	OpCall       = "call"
)

// ToolCallArgs names a tool of an extension and its JSON arguments.
type ToolCallArgs struct {
	ExtensionID string          `json:"extensionId"`
	Tool        string          `json:"tool"`
	Args        json.RawMessage `json:"args,omitempty"`
}

// ToolPlan is a tool call the host prepared before anyone approves it: the tool with the class the host enforces, and
// the arguments as the host checked them, and the line the extension wrote for what the call will do. Host-only; extensions never read it.
type ToolPlan struct {
	Tool        api.AIToolRecord
	Args        json.RawMessage
	Description string
}

// AIToolInvocation carries a tool call to the extension: the tool's name in Ref.ID and its JSON arguments in Value.
func AIToolInvocation(call api.AIToolCall) *api.Invocation {
	return &api.Invocation{Ref: api.ActionRef{ID: call.Name}, Value: string(call.Args)}
}

// AIToolCallOf reads the tool call AIToolInvocation carried.
func AIToolCallOf(invocation *api.Invocation) api.AIToolCall {
	if invocation == nil {
		return api.AIToolCall{}
	}
	return api.AIToolCall{Name: invocation.Ref.ID, Args: json.RawMessage(invocation.Value)}
}

// userApprovedKey marks a request the user already allowed.
type userApprovedKey struct{}

// WithUserApproval marks ctx as serving a call the user allowed, such as a destructive AI tool call. The services of
// that request do not ask again. Only the host sets it, on the context of the request it sends.
func WithUserApproval(ctx context.Context) context.Context {
	return context.WithValue(ctx, userApprovedKey{}, true)
}

// UserApproved reports whether WithUserApproval marked ctx.
func UserApproved(ctx context.Context) bool {
	approved, _ := ctx.Value(userApprovedKey{}).(bool)
	return approved
}

// Question is the approval question of the call: the extension and, in its words, what the call will do. A tool that
// describes nothing is named with its arguments.
func (p ToolPlan) Question() string {
	what := p.Description
	if what == "" {
		what = fmt.Sprintf("Run “%s” with %s", p.Tool.Name, clipRunes(string(p.Args), 80))
	}
	return p.Tool.ExtensionTitle + ": " + what
}

func clipRunes(text string, limit int) string {
	if runes := []rune(text); len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return text
}

// ToolFailure is a failure the extension's tool returned, as an error that keeps its code and message.
type ToolFailure struct {
	Failure api.Failure
}

func (e ToolFailure) Error() string { return e.Failure.Message }

// Is lets api.FailureOf read the failure's code back.
func (e ToolFailure) Is(target error) bool {
	return target == failureSentinels[e.Failure.Code]
}

var failureSentinels = map[api.ErrorCode]error{
	api.CodeUnavailable: api.ErrNotAvailable, api.CodeUnsupported: api.ErrNotAvailable, api.CodePermission: api.ErrPermission,
	api.CodeInvalid: api.ErrInvalidInput, api.CodeNotFound: api.ErrNotFound, api.CodeQuota: api.ErrQuotaExceeded,
}
