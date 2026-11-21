package chat

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// TestAllowedCallRunsAfterTheUserAllowsIt checks the confirmation path: the row waits, then runs once allowed.
func TestAllowedCallRunsAfterTheUserAllowsIt(t *testing.T) {
	var runs atomic.Int32
	backend := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "d1", Name: "delete", Arguments: []string{`{"text":"old"}`}}),
		fixtures.TextReply("Deleted."),
	)
	f := newScriptedFixture(t, toolsOf(t, deleteTool(&runs)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "delete old"); err != nil {
		t.Fatal(err)
	}
	f.awaitState(func(messages []Message) bool { return hasToolWith(messages, StatusAwaiting) })
	if runs.Load() != 0 {
		t.Fatal("the confirmed call ran before the user allowed it")
	}
	if err := f.svc.Decide(chat.ID, true); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	f.awaitTurn(chat.ID)
	if runs.Load() != 1 || !hasToolWith(f.messages(chat.ID), StatusComplete) {
		t.Fatalf("runs = %d after an allow", runs.Load())
	}
}

// TestRefusedCallNeverRunsAndTheModelIsTold checks that a refusal reaches the model as the user's answer.
func TestRefusedCallNeverRunsAndTheModelIsTold(t *testing.T) {
	var runs atomic.Int32
	backend := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "d1", Name: "delete", Arguments: []string{`{"text":"old"}`}}),
		fixtures.TextReply("Okay, I kept it."),
	)
	f := newScriptedFixture(t, toolsOf(t, deleteTool(&runs)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "delete old"); err != nil {
		t.Fatal(err)
	}
	f.awaitState(func(messages []Message) bool { return hasToolWith(messages, StatusAwaiting) })
	if err := f.svc.Decide(chat.ID, false); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	f.awaitTurn(chat.ID)
	if runs.Load() != 0 || !hasToolWith(f.messages(chat.ID), StatusRefused) {
		t.Fatalf("runs = %d after a refusal", runs.Load())
	}
	if !strings.Contains(backend.Requests()[1], "The user did not allow this tool call.") {
		t.Fatal("the model was not told the call was refused")
	}
}

// TestDecideWithoutAPendingCallIsRefused checks the error when nothing waits for an answer.
func TestDecideWithoutAPendingCallIsRefused(t *testing.T) {
	f := newScriptedFixture(t, Toolset{}, fixtures.NewScriptedBackend())
	chat := f.newChat()
	if err := f.svc.Decide(chat.ID, true); err != ErrNothingToDecide {
		t.Fatalf("Decide = %v, want ErrNothingToDecide", err)
	}
}

// TestStopWhileAwaitingAnswerCancelsTheTurn checks Escape during a confirmation: nothing runs, the row is cancelled.
func TestStopWhileAwaitingAnswerCancelsTheTurn(t *testing.T) {
	var runs atomic.Int32
	backend := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "d1", Name: "delete", Arguments: []string{`{"text":"old"}`}}),
	)
	f := newScriptedFixture(t, toolsOf(t, deleteTool(&runs)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "delete old"); err != nil {
		t.Fatal(err)
	}
	f.awaitState(func(messages []Message) bool { return hasToolWith(messages, StatusAwaiting) })
	f.svc.Stop(chat.ID)
	f.awaitTurn(chat.ID)
	rows := toolRows(f.messages(chat.ID))
	if runs.Load() != 0 || len(rows) != 1 || rows[0].Status != StatusCancelled {
		t.Fatalf("rows = %+v, runs = %d", rows, runs.Load())
	}
	if f.settled[len(f.settled)-1].Ending != EndStopped {
		t.Fatalf("the turn ended %+v, want stopped", f.settled[len(f.settled)-1])
	}
}

// TestStopWhileAToolRunsCancelsItsRow checks that a tool which waits on its context is cancelled with the turn.
func TestStopWhileAToolRunsCancelsItsRow(t *testing.T) {
	started := make(chan struct{})
	backend := fixtures.NewScriptedBackend(
		fixtures.ToolCallReply("", fixtures.ToolCall{ID: "w1", Name: "wait", Arguments: []string{`{}`}}),
	)
	f := newScriptedFixture(t, toolsOf(t, blockingTool(started)), backend)
	chat := f.newChat()
	if err := f.svc.Send(chat.ID, "wait"); err != nil {
		t.Fatal(err)
	}
	<-started
	f.svc.Stop(chat.ID)
	f.awaitTurn(chat.ID)
	if rows := toolRows(f.messages(chat.ID)); len(rows) != 1 || rows[0].Status != StatusCancelled {
		t.Fatalf("rows = %+v", rows)
	}
}
