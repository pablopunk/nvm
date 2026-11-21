package chat

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// programTool stands in for run_program: it shows the code it was given, does one thing for the user under a nested row
// and asks before it.
func programTool(effects *atomic.Int32) agent.Tool {
	return agent.Tool{
		Name: "program", Label: "Running a program", Timeout: -1, Schema: toolSchema,
		Summarize: func(args json.RawMessage) string {
			var in struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(args, &in)
			return in.Text
		},
		Run: func(ctx context.Context, _ json.RawMessage) (agent.Result, error) {
			step := agent.SubCallsFrom(ctx).Begin("Deleting a note", "Delete note “Old”")
			allowed, err := step.Ask(ctx)
			if err != nil {
				step.End(agent.CallCancelled, "Stopped before it ran", "")
				return agent.Result{}, err
			}
			if !allowed {
				step.End(agent.CallRefused, "Refused by you", "The user did not allow this.")
				return agent.Result{Text: "The user declined.", Outcome: "1 declined by you"}, nil
			}
			effects.Add(1)
			step.End(agent.CallRan, "Deleted note “Old”", "deleted note old")
			return agent.Result{Text: "Result: ok\ndeleted note old", Outcome: "1 change"}, nil
		},
	}
}

func newProgramFixture(t *testing.T, effects *atomic.Int32) (*fixture, *fixtures.ScriptedBackend, Chat) {
	t.Helper()
	backend := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "p1", Name: "program", Arguments: []string{`{"text":"ctx.Notes().Delete(id)"}`}}),
		fixtures.TextReply("Deleted."),
	)
	f := newScriptedFixture(t, toolsOf(t, programTool(effects)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "delete my old note"); err != nil {
		t.Fatal(err)
	}
	return f, backend, chat
}

// The approval row of a nested call is the one the chat asks about: Decide answers it, and the program row stays above it.
func TestNestedApprovalRowIsAnsweredThroughDecide(t *testing.T) {
	var effects atomic.Int32
	f, backend, chat := newProgramFixture(t, &effects)
	f.awaitState(func(messages []Message) bool { return hasToolWith(messages, StatusAwaiting) })
	if effects.Load() != 0 {
		t.Fatal("the effect ran before the user answered")
	}
	rows := toolRows(f.messages(chat.ID))
	if len(rows) != 2 || rows[0].Tool.CallID != "p1" || rows[0].Status != StatusRunning || rows[0].Tool.Input != "ctx.Notes().Delete(id)" {
		t.Fatalf("the program row = %+v", rows)
	}
	if nested := rows[1]; nested.Status != StatusAwaiting || nested.Tool.ParentID != "p1" || nested.Tool.Label != "Deleting a note" || nested.Tool.Input != "Delete note “Old”" {
		t.Fatalf("the nested row = %+v, %+v", nested, nested.Tool)
	}
	if err := f.svc.Decide(chat.ID, true); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	f.awaitTurn(chat.ID)
	rows = toolRows(f.messages(chat.ID))
	if effects.Load() != 1 || rows[0].Status != StatusComplete || rows[0].Tool.Outcome != "1 change" || rows[1].Status != StatusComplete || rows[1].Text != "deleted note old" {
		t.Fatalf("effects %d, rows %+v", effects.Load(), rows)
	}
	second := backend.Requests()[1]
	if strings.Count(second, `"role":"tool"`) != 1 || !strings.Contains(second, "deleted note old") {
		t.Fatalf("the next request must carry the program's result once: %s", second)
	}
}

func TestNestedRowsSurviveAReloadWithTheirParent(t *testing.T) {
	var effects atomic.Int32
	f, _, chat := newProgramFixture(t, &effects)
	f.awaitState(func(messages []Message) bool { return hasToolWith(messages, StatusAwaiting) })
	if err := f.svc.Decide(chat.ID, true); err != nil {
		t.Fatal(err)
	}
	f.awaitTurn(chat.ID)
	f.svc.Reload()
	rows := toolRows(f.messages(chat.ID))
	if len(rows) != 2 || rows[1].Tool.ParentID != "p1" || rows[0].Tool.Input == "" || rows[0].Text == "" {
		t.Fatalf("rows after a reload = %+v", rows)
	}
}

func TestStopWhileANestedRowAwaitsCancelsBothRows(t *testing.T) {
	var effects atomic.Int32
	f, _, chat := newProgramFixture(t, &effects)
	f.awaitState(func(messages []Message) bool { return hasToolWith(messages, StatusAwaiting) })
	f.svc.Stop(chat.ID)
	f.awaitTurn(chat.ID)
	rows := toolRows(f.messages(chat.ID))
	if effects.Load() != 0 || len(rows) != 2 || rows[0].Status != StatusCancelled || rows[1].Status != StatusCancelled {
		t.Fatalf("effects %d, rows %+v", effects.Load(), rows)
	}
}
