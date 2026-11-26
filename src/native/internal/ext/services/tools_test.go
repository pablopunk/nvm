package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// recordingTools is a tool host with one tool of each kind that matters here, which records the calls it runs.
type recordingTools struct {
	ran      []string
	approved []bool
}

var toolClasses = map[string]api.AIToolClass{"forecast": api.AIToolRead, "history": api.AIToolSensitiveRead, "forget": api.AIToolDestructive}

func (r *recordingTools) List(string) []api.AIToolRecord {
	return []api.AIToolRecord{{ExtensionID: "acme.weather", Name: "forecast", Class: api.AIToolRead}}
}

func (r *recordingTools) Plan(_ context.Context, extensionID, tool string, args json.RawMessage) (protocol.ToolPlan, error) {
	record := api.AIToolRecord{ExtensionID: extensionID, ExtensionTitle: "Weather", Name: tool, Class: toolClasses[tool]}
	return protocol.ToolPlan{Tool: record, Args: args, Description: "Forget the saved cities"}, nil
}

func (r *recordingTools) Call(_ context.Context, _, tool string, _ json.RawMessage, approved bool) (json.RawMessage, error) {
	r.ran = append(r.ran, tool)
	r.approved = append(r.approved, approved)
	return json.RawMessage(`{"ok":true}`), nil
}

func serveTool(h *Handler, method, tool string) protocol.ServiceReply {
	raw, _ := json.Marshal(protocol.ToolCallArgs{ExtensionID: "acme.weather", Tool: tool})
	ctx := protocol.WithRequestMethod(context.Background(), method)
	return h.Serve(ctx, protocol.ServiceCall{CallID: 1, Service: protocol.ServiceTools, Op: protocol.OpCall, Args: raw})
}

func toolCaller(tools AIToolHost, questions *[]Question, answer bool) *Handler {
	confirm := func(_ context.Context, question Question) (bool, error) {
		*questions = append(*questions, question)
		return answer, nil
	}
	return New(Deps{Tools: tools, Confirm: confirm}, manifestFor("acme.caller", api.CapabilityTools))
}

func TestToolsNeedTheToolsCapabilityAndAPaletteRunToCall(t *testing.T) {
	tools := &recordingTools{}
	undeclared := New(Deps{Tools: tools}, manifestFor("acme.other"))
	requireCode(t, undeclared.Serve(context.Background(), protocol.ServiceCall{Service: protocol.ServiceTools, Op: protocol.OpList}), api.CodePermission, "tools")
	var questions []Question
	caller := toolCaller(tools, &questions, true)
	requireCode(t, serveTool(caller, protocol.MethodSearch, "forecast"), api.CodePermission, "palette action")
	if len(tools.ran) != 0 {
		t.Fatalf("a search ran a tool: %v", tools.ran)
	}
	mustOK(t, serveTool(caller, protocol.MethodRun, "forecast"))
	if len(questions) != 0 || len(tools.ran) != 1 || tools.approved[0] {
		t.Fatalf("a read asked %v, ran %v approved %v", questions, tools.ran, tools.approved)
	}
}

// A destructive tool asks the palette with the extension's description, and runs with the user's approval.
func TestADestructiveToolAsksAndRunsApproved(t *testing.T) {
	tools := &recordingTools{}
	var questions []Question
	mustOK(t, serveTool(toolCaller(tools, &questions, true), protocol.MethodRun, "forget"))
	if len(questions) != 1 || questions[0].Action != "Weather: Forget the saved cities" || !tools.approved[0] {
		t.Fatalf("questions %+v, approved %v", questions, tools.approved)
	}
}

func TestADeclinedToolRunsNothing(t *testing.T) {
	tools := &recordingTools{}
	var questions []Question
	requireCode(t, serveTool(toolCaller(tools, &questions, false), protocol.MethodRun, "forget"), api.CodePermission, "cancelled")
	if len(tools.ran) != 0 {
		t.Fatalf("a declined tool ran: %v", tools.ran)
	}
}

// A sensitive read asks, but the answer approves only reading: the tool's own service calls still ask before removing.
func TestASensitiveReadAsksButIsNotApprovedToRemove(t *testing.T) {
	tools := &recordingTools{}
	var questions []Question
	mustOK(t, serveTool(toolCaller(tools, &questions, true), protocol.MethodRun, "history"))
	if len(questions) != 1 || tools.approved[0] {
		t.Fatalf("questions %+v, approved %v", questions, tools.approved)
	}
}

func TestAnExtensionCannotCallItsOwnToolsThroughTheService(t *testing.T) {
	weather := New(Deps{Tools: &recordingTools{}}, manifestFor("acme.weather", api.CapabilityTools))
	requireCode(t, serveTool(weather, protocol.MethodRun, "forecast"), api.CodeInvalid, "own tools")
}

// A request the user already allowed does not ask again when the tool removes data.
func TestAnApprovedRequestDoesNotAskAgain(t *testing.T) {
	asked := 0
	h := New(Deps{Confirm: func(context.Context, Question) (bool, error) { asked++; return false, nil }}, manifestFor("acme.weather"))
	if err := h.confirmDestructive(protocol.WithUserApproval(context.Background()), "Remove 3 clipboard items"); err != nil || asked != 0 {
		t.Fatalf("err = %v, asked %d", err, asked)
	}
	if err := h.confirmDestructive(context.Background(), "Remove 3 clipboard items"); err == nil || asked != 1 {
		t.Fatalf("an unapproved request did not ask: err = %v, asked %d", err, asked)
	}
}
