package codemode

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
)

const deleteOld = `found, err := ctx.Notes().Search("old ideas", 1)
if err != nil || len(found) == 0 { return "no such note", err }
err = ctx.Notes().Delete(found[0].ID)
if err != nil { return "refused: " + err.Error(), nil }
return "deleted", nil`

// The approval is raised before the store changes, and says what will be deleted from the store's own data.
func TestDeleteAsksFirstWithAHostBuiltQuestionAndRunsAfterAllow(t *testing.T) {
	f := newFixture(t)
	doomed := f.note("# Old ideas\n- a boat")
	f.steps.decide = func(row *fakeRow, _ context.Context) (bool, error) {
		if _, still := f.notes.Get(doomed.ID); !still {
			t.Error("the note was deleted before the user answered")
		}
		return true, nil
	}
	result := f.run(deleteOld)
	if _, found := f.notes.Get(doomed.ID); found {
		t.Fatal("the note survived an allowed delete")
	}
	contains(t, result.Text, `Result: "deleted"`, "deleted note "+doomed.ID+" “Old ideas” (the user allowed it)")
	var asked fakeRow
	for _, row := range f.steps.snapshot() {
		if row.asked {
			asked = row
		}
	}
	if asked.label != "Deleting a note" || !strings.Contains(asked.detail, "Delete note “Old ideas”, edited just now") {
		t.Fatalf("the question = %+v", asked)
	}
}

func TestDeclinedDeleteLeavesTheNoteAndTheProgramGetsARefusal(t *testing.T) {
	f := newFixture(t)
	kept := f.note("# Old ideas")
	f.steps.decide = decline
	result := f.run(deleteOld)
	if _, found := f.notes.Get(kept.ID); !found {
		t.Fatal("a declined delete removed the note")
	}
	contains(t, result.Text, "the user declined this action", "the user declined, so it was not done")
	if result.Outcome != "1 declined by you" && !strings.Contains(result.Outcome, "declined by you") {
		t.Fatalf("outcome = %q", result.Outcome)
	}
	rows := f.steps.snapshot()
	if last := rows[len(rows)-1]; last.status != agent.CallRefused {
		t.Fatalf("the delete row ended %q; want refused", last.status)
	}
}

// A chat that cannot ask never deletes.
func TestDeleteWithoutAChatToAskIsRefused(t *testing.T) {
	f := newFixture(t)
	kept := f.note("# Old ideas")
	result, err := f.runner.Run(context.Background(), deleteOld)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := f.notes.Get(kept.ID); !found {
		t.Fatal("a delete ran with nobody to ask")
	}
	contains(t, result.Text, "this action needs the user's approval and this chat cannot ask for it")
}

// An approval that is not answered in time is a refusal, not a delete.
func TestUnansweredApprovalTimesOutAsARefusal(t *testing.T) {
	f := newFixture(t, func(o *Options) { o.ApprovalWait = 100 * time.Millisecond })
	kept := f.note("# Old ideas")
	f.steps.decide = func(_ *fakeRow, ctx context.Context) (bool, error) {
		<-ctx.Done()
		return false, ctx.Err()
	}
	result := f.run(deleteOld)
	if _, found := f.notes.Get(kept.ID); !found {
		t.Fatal("an unanswered approval deleted the note")
	}
	contains(t, result.Text, "did not answer in time")
	if rows := f.steps.snapshot(); rows[len(rows)-1].outcome != "No answer in time" {
		t.Fatalf("rows = %+v", rows)
	}
}

// Second line of defence: the extension services behind the gate refuse a delete the gate did not allow.
func TestServicesRefuseADeleteTheGateDidNotAllow(t *testing.T) {
	f := newFixture(t)
	doomed := f.note("# Old ideas")
	g := newGate(gateConfig{rules: notesRules(), runCtx: context.Background(), notes: f.runner.opts.Notes})
	handler := services.New(services.Deps{Clock: f.runner.opts.Supervisor.Clock, Notes: f.runner.opts.Notes, Confirm: g.confirm},
		api.Manifest{Identity: Identity, Capabilities: capabilities})
	defer func() { _ = handler.Close() }()
	ctx := protocol.WithRequestMethod(context.Background(), protocol.MethodProgram)
	args := []byte(`{"key":"` + doomed.ID + `"}`)
	reply := handler.Serve(ctx, protocol.ServiceCall{Service: protocol.ServiceNotes, Op: protocol.OpDelete, Args: args})
	if reply.Failure == nil {
		t.Fatal("the service deleted without the gate's allowance")
	}
	if _, found := f.notes.Get(doomed.ID); !found {
		t.Fatal("the note is gone")
	}
	g.allowed = &key{protocol.ServiceNotes, protocol.OpDelete}
	if reply := handler.Serve(ctx, protocol.ServiceCall{Service: protocol.ServiceNotes, Op: protocol.OpDelete, Args: args}); reply.Failure != nil {
		t.Fatalf("an allowed delete failed: %+v", reply.Failure)
	}
	if _, found := f.notes.Get(doomed.ID); found {
		t.Fatal("an allowed delete left the note")
	}
}

// An allowance covers one call: a second delete in the same program asks again.
func TestEachDeleteAsksAgain(t *testing.T) {
	f := newFixture(t)
	f.note("# One")
	f.note("# Two")
	questions := 0
	f.steps.decide = func(*fakeRow, context.Context) (bool, error) { questions++; return true, nil }
	f.run(`list, err := ctx.Notes().List()
if err != nil { return nil, err }
for _, n := range list { if err := ctx.Notes().Delete(n.ID); err != nil { return nil, err } }
return len(list), nil`)
	if questions != 2 || len(f.noteIDs()) != 0 {
		t.Fatalf("questions = %d, notes left = %v", questions, f.noteIDs())
	}
}
