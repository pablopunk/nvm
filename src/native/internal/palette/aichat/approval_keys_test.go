package aichat

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// deleteTool is a tool that asks the user before it runs and counts its runs.
func deleteTool(runs *atomic.Int32) agent.Tool {
	return agent.Tool{
		Name: "delete", Label: "Deleting", Confirm: true,
		Schema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"additionalProperties":false}`),
		Run: func(context.Context, json.RawMessage) (agent.Result, error) {
			runs.Add(1)
			return agent.Result{Text: "deleted", Outcome: "Deleted"}, nil
		},
	}
}

// scriptedClient is a backend client that answers chat requests from a recorded backend.
func scriptedClient(t *testing.T, handler *fixtures.ScriptedBackend) *backend.Client {
	t.Helper()
	set := fake.NewSet()
	set.HTTP.Handle(handler)
	logger, err := logx.New(set.FS, set.Clock, "/logs", logx.Options{FlushInterval: time.Second, Level: logx.Debug})
	if err != nil {
		t.Fatalf("logx: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil {
		t.Fatalf("environment: %v", err)
	}
	client, err := backend.New(set.HTTP, logger, env, keyToken("test-token"),
		backend.WithClock(set.Clock), backend.WithRetryPolicy(1, 0, 0))
	if err != nil {
		t.Fatalf("backend: %v", err)
	}
	return client
}

// awaitAwaiting settles queued work until the chat waits for an answer to a tool call.
func (r *rig) awaitAwaiting(id string) {
	r.t.Helper()
	deadline := time.After(watchdog)
	for {
		r.settle()
		if r.service.Awaiting(id) {
			return
		}
		select {
		case <-r.notify:
		case <-deadline:
			r.t.Fatal("no tool call ever waited for an answer")
		}
	}
}

// startDelete sends a request that makes the model ask for the confirmed tool, and waits until the page waits for it.
func startDelete(t *testing.T, runs *atomic.Int32, model *fixtures.ScriptedBackend) (*rig, chat.Chat, *palette.View, *ui.Tester) {
	t.Helper()
	r := newRigWithTools(t, scriptedClient(t, model), chat.Toolset{Registry: registryOf(t, deleteTool(runs)), Policy: agent.AllowAll{}})
	record := r.chatWith("", chat.TierFast)
	view := ChatView(r.deps(), record.ID)
	r.palette.Show()
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	tt.Type("delete it")
	tt.Key(0, ui.KeyEnter)
	r.awaitAwaiting(record.ID)
	return r, record, view, tt
}

// registryOf builds the tools of a test.
func registryOf(t *testing.T, tools ...agent.Tool) *agent.Registry {
	t.Helper()
	registry, err := agent.NewRegistry(tools...)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return registry
}

// TestEnterAllowsTheWaitingCallAndTheFooterSaysSo checks that Enter on an empty composer allows a waiting call, that the
// footer's primary action says Allow while it waits, and that the action menu offers Refuse.
func TestEnterAllowsTheWaitingCallAndTheFooterSaysSo(t *testing.T) {
	var runs atomic.Int32
	model := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "d1", Name: "delete", Arguments: []string{`{"text":"x"}`}}),
		fixtures.TextReply("Deleted it."),
	)
	r, record, view, tt := startDelete(t, &runs, model)
	if primary := view.Primary(); primary.Title != "Allow" {
		t.Fatalf("footer while waiting = %q, want Allow", primary.Title)
	}
	if !menuHasTitle(view, "Refuse") {
		t.Fatal("the action menu does not offer Refuse while a call waits")
	}
	tt.Key(0, ui.KeyEnter)
	r.awaitIdle(record.ID)
	if runs.Load() != 1 {
		t.Fatalf("tool runs = %d after Enter, want 1", runs.Load())
	}
	if primary := view.Primary(); primary.Title != "Send" {
		t.Fatalf("footer after the answer = %q, want Send", primary.Title)
	}
}

// menuHasTitle reports whether the action menu of view has an action titled title.
func menuHasTitle(view *palette.View, title string) bool {
	for _, a := range view.Actions() {
		if a.Title == title {
			return true
		}
	}
	return false
}

// TestCommandDRefusesTheWaitingCall checks that Command with D refuses the waiting call, so nothing runs and the reply
// goes on.
func TestCommandDRefusesTheWaitingCall(t *testing.T) {
	var runs atomic.Int32
	model := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "d1", Name: "delete", Arguments: []string{`{"text":"x"}`}}),
		fixtures.TextReply("Kept it."),
	)
	r, record, _, tt := startDelete(t, &runs, model)
	tt.Key(palette.Mod, ui.KeyD)
	r.awaitIdle(record.ID)
	if runs.Load() != 0 {
		t.Fatalf("tool runs = %d after a refusal, want 0", runs.Load())
	}
	if last := lastReply(mustChat(t, r, record.ID)); last != "Kept it." {
		t.Fatalf("the reply after the refusal = %q", last)
	}
}
