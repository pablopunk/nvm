package codemode

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// turnFixture runs real agent turns whose model calls run_program, over the fixture's runner and notes.
type turnFixture struct {
	*fixture
	model *agent.ScriptedModel
	setup agent.Setup
}

func newTurnFixture(t *testing.T, code string, tweak ...fixtureOption) *turnFixture {
	t.Helper()
	f := newFixture(t, tweak...)
	registry, err := agent.NewRegistry(Tool(f.runner))
	if err != nil {
		t.Fatal(err)
	}
	arguments, _ := json.Marshal(map[string]string{"code": code})
	delta := backend.ToolCallDelta{Index: 0, ID: "p1", Name: ToolName, ArgumentsDelta: string(arguments)}
	model := agent.NewScriptedModel(
		agent.ScriptedTurn{Chunks: []agent.ModelChunk{{ToolDelta: &delta}}},
		agent.ScriptedTurn{Chunks: []agent.ModelChunk{{Text: "Done."}}},
	)
	return &turnFixture{fixture: f, model: model, setup: agent.Setup{Model: model, Tools: registry, Clock: fake.NewClock()}}
}

func (f *turnFixture) start() *agent.Turn {
	history := []backend.Message{{Role: backend.RoleUser, Content: backend.TextContent("do it")}}
	return agent.Start(context.Background(), f.setup, history)
}

// drive collects a turn's events and answers each approval with answer, which may also stop the turn.
func drive(turn *agent.Turn, answer func(agent.Event)) []agent.Event {
	var events []agent.Event
	for event := range turn.Events() {
		events = append(events, event)
		if event.Kind == agent.EventApproval {
			answer(event)
		}
	}
	return events
}

// endOf is the end event of the nested call with the label.
func endOf(t *testing.T, events []agent.Event, label string) agent.Event {
	t.Helper()
	for _, event := range ofKind(events, agent.EventToolEnd) {
		if event.Call.Label == label {
			return event
		}
	}
	t.Fatalf("no call ended with the label %q", label)
	return agent.Event{}
}

func ofKind(events []agent.Event, kind agent.EventKind) []agent.Event {
	var out []agent.Event
	for _, event := range events {
		if event.Kind == kind {
			out = append(out, event)
		}
	}
	return out
}

// toolResult is the text the model read for its run_program call on its second turn.
func (f *turnFixture) toolResult(t *testing.T) string {
	t.Helper()
	requests := f.model.Requests()
	if len(requests) < 2 {
		t.Fatalf("the model was asked %d times; want 2", len(requests))
	}
	for _, message := range requests[1].Messages {
		if message.Role == backend.RoleTool {
			return string(message.Content)
		}
	}
	t.Fatal("the second model request carries no tool result")
	return ""
}

func TestTurnCreatesANoteWithNestedRowsAndTheModelReadsTheJournal(t *testing.T) {
	f := newTurnFixture(t, `n, err := ctx.Notes().CreateWith("# Packing list")
if err != nil { return nil, err }
return n.ID, nil`)
	events := drive(f.start(), func(agent.Event) { t.Fatal("a create asked the user") })
	starts := ofKind(events, agent.EventToolStart)
	if len(starts) != 2 || starts[0].Call.ID != "p1" || starts[0].Call.Label != "Running a program" ||
		starts[1].Call.ParentID != "p1" || starts[1].Call.Label != "Creating a note" {
		t.Fatalf("starts = %+v", starts)
	}
	if !strings.Contains(starts[0].Call.Input, "CreateWith") {
		t.Fatalf("the program row does not show the code: %q", starts[0].Call.Input)
	}
	ends := ofKind(events, agent.EventToolEnd)
	if len(ends) != 2 || ends[1].Call.ID != "p1" || ends[1].Status != agent.CallRan || ends[1].Outcome != "1 change" {
		t.Fatalf("ends = %+v", ends)
	}
	if got := f.notes.List(); len(got) != 1 {
		t.Fatalf("notes = %+v", got)
	}
	contains(t, f.toolResult(t), "Result:", "created note")
}

func TestTurnAsksForADeleteAndRunsItAfterTheUserAllows(t *testing.T) {
	f := newTurnFixture(t, deleteOld)
	note := f.note("# Old ideas")
	turn := f.start()
	events := drive(turn, func(event agent.Event) {
		if event.Call.ParentID != "p1" || event.Call.Label != "Deleting a note" || !strings.Contains(event.Call.Input, "Delete note “Old ideas”") {
			t.Errorf("approval for %+v", event.Call)
		}
		if _, found := f.notes.Get(note.ID); !found {
			t.Error("the note was gone before the user answered")
		}
		if !turn.Decide(event.Call.ID, true) {
			t.Error("nothing waited for the answer")
		}
	})
	if _, found := f.notes.Get(note.ID); found {
		t.Fatal("the allowed delete did not run")
	}
	if terminal := events[len(events)-1]; terminal.Kind != agent.EventDone {
		t.Fatalf("terminal = %s", terminal.Kind)
	}
	contains(t, f.toolResult(t), "deleted note "+note.ID)
}

func TestTurnDeclinedDeleteKeepsTheNoteAndTheModelHearsIt(t *testing.T) {
	f := newTurnFixture(t, deleteOld)
	note := f.note("# Old ideas")
	turn := f.start()
	events := drive(turn, func(event agent.Event) { turn.Decide(event.Call.ID, false) })
	if _, found := f.notes.Get(note.ID); !found {
		t.Fatal("a declined delete removed the note")
	}
	if end := endOf(t, events, "Deleting a note"); end.Status != agent.CallRefused || end.Outcome != "Refused by you" {
		t.Fatalf("the nested row ended %+v", end)
	}
	contains(t, f.toolResult(t), "the user declined")
}

// Stop while the question is up: the note stays, both rows end cancelled, the turn aborts and no child is left.
func TestStopWhileTheQuestionIsUpLeavesTheNoteAndNoProcess(t *testing.T) {
	f := newTurnFixture(t, deleteOld)
	note := f.note("# Old ideas")
	turn := f.start()
	events := drive(turn, func(agent.Event) { turn.Cancel() })
	if _, found := f.notes.Get(note.ID); !found {
		t.Fatal("stopping the chat deleted the note")
	}
	if end := endOf(t, events, "Deleting a note"); end.Status != agent.CallCancelled {
		t.Fatalf("the nested row ended %+v", end)
	}
	if end := endOf(t, events, "Running a program"); end.Status != agent.CallCancelled {
		t.Fatalf("the program row ended %+v", end)
	}
	if terminal := events[len(events)-1]; terminal.Kind != agent.EventAborted {
		t.Fatalf("terminal = %s", terminal.Kind)
	}
	noChildren(t)
	f.noStagedPrograms()
}

// An allow that arrives at the same instant as Stop must not delete: the gate looks at the stop after the answer.
func TestAllowThatRacesStopDeletesNothing(t *testing.T) {
	f := newFixture(t)
	note := f.note("# Old ideas")
	ctx, stop := context.WithCancel(context.Background())
	f.steps.decide = func(*fakeRow, context.Context) (bool, error) {
		stop()
		return true, nil
	}
	_, err := f.runner.Run(agent.WithSubCalls(ctx, f.steps), deleteOld)
	if err == nil {
		t.Fatal("a stopped run reported success")
	}
	if _, found := f.notes.Get(note.ID); !found {
		t.Fatal("an allow that raced Stop deleted the note")
	}
	noChildren(t)
}

func TestStopWhileTheProgramComputesEndsItAtOnce(t *testing.T) {
	f := newTurnFixture(t, "for {\n}\nreturn nil, nil")
	turn := f.start()
	go func() {
		time.Sleep(300 * time.Millisecond)
		turn.Cancel()
	}()
	started := time.Now()
	events := drive(turn, func(agent.Event) {})
	if took := time.Since(started); took > 5*time.Second {
		t.Fatalf("Stop took %s to end the run", took)
	}
	if terminal := events[len(events)-1]; terminal.Kind != agent.EventAborted {
		t.Fatalf("terminal = %s", terminal.Kind)
	}
	noChildren(t)
	f.noStagedPrograms()
}

// The tool has no wall-clock timeout: a slow answer does not end the program the way a slow tool would end.
func TestToolTimeoutDoesNotCountTheUsersAnswer(t *testing.T) {
	f := newTurnFixture(t, deleteOld)
	f.setup.ToolTimeout = 200 * time.Millisecond
	note := f.note("# Old ideas")
	turn := f.start()
	drive(turn, func(event agent.Event) {
		time.Sleep(600 * time.Millisecond)
		turn.Decide(event.Call.ID, true)
	})
	if _, found := f.notes.Get(note.ID); found {
		t.Fatal("the delete did not run after a slow answer")
	}
}

// What the model reads is capped by the agent loop, whatever the program returned.
func TestModelResultIsCappedAtTheLoopsOutputLimit(t *testing.T) {
	f := newTurnFixture(t, "return strings.Repeat(\"x\", 60000), nil")
	drive(f.start(), func(agent.Event) {})
	if got := len(f.toolResult(t)); got > agent.DefaultMaxOutputBytes+64 {
		t.Fatalf("the model read %d bytes", got)
	}
}
