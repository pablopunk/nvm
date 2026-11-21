package codemode

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
)

func TestProgramCreatesANoteAndTheModelReadsWhatHappened(t *testing.T) {
	f := newFixture(t)
	result := f.run("note, err := ctx.Notes().CreateWith(\"# Packing list\\n- socks\")\nif err != nil { return nil, err }\nreturn note.ID, nil")
	if result.Failed {
		t.Fatalf("failed: %s", result.Text)
	}
	list := f.notes.List()
	if len(list) != 1 || list[0].Markdown != "# Packing list\n- socks" {
		t.Fatalf("notes = %+v", list)
	}
	contains(t, result.Text, "Result: \""+list[0].ID+"\"", "created note "+list[0].ID+" “Packing list”")
	rows := f.steps.snapshot()
	if len(rows) != 1 || rows[0].asked || rows[0].status != agent.CallRan || rows[0].label != "Creating a note" {
		t.Fatalf("rows = %+v; want one note row that did not ask", rows)
	}
	if result.Outcome != "1 change" {
		t.Fatalf("outcome = %q", result.Outcome)
	}
}

func TestProgramFindsReadsAndAppendsWithoutAsking(t *testing.T) {
	f := newFixture(t)
	shopping := f.note("# Shopping\n- eggs")
	f.note("# Ideas\n- a boat")
	result := f.run(`found, err := ctx.Notes().Search("shopping", 5)
if err != nil || len(found) == 0 { return "no such note", err }
note, ok, err := ctx.Notes().Get(found[0].ID)
if err != nil || !ok { return nil, err }
_, conflict, err := ctx.Notes().Update(note.ID, note.Markdown+"\n- milk", note.UpdatedAt)
return conflict, err`)
	if result.Failed {
		t.Fatalf("failed: %s", result.Text)
	}
	got, _ := f.notes.Get(shopping.ID)
	if got.Markdown != "# Shopping\n- eggs\n- milk" {
		t.Fatalf("note = %q", got.Markdown)
	}
	contains(t, result.Text, "Result: false", "saved note "+shopping.ID, "2 read-only calls")
	for _, row := range f.steps.snapshot() {
		if row.asked {
			t.Fatalf("%q asked the user", row.label)
		}
	}
}

func TestProgramEditsAndListsNotes(t *testing.T) {
	f := newFixture(t)
	note := f.note("old")
	result := f.run(`list, err := ctx.Notes().List()
if err != nil { return nil, err }
n, _, err := ctx.Notes().Update(list[0].ID, "new text", list[0].UpdatedAt)
return n.Markdown, err`)
	contains(t, result.Text, `Result: "new text"`)
	if got, _ := f.notes.Get(note.ID); got.Markdown != "new text" {
		t.Fatalf("note = %q", got.Markdown)
	}
}

// A write made on a note that changed since it was read saves nothing and says so to the model.
func TestProgramEditConflictSavesNothing(t *testing.T) {
	f := newFixture(t)
	note := f.note("original")
	result := f.run(`n, _, err := ctx.Notes().Get("` + note.ID + `")
if err != nil { return nil, err }
_, _, err = ctx.Notes().Update(n.ID, "first edit", n.UpdatedAt)
if err != nil { return nil, err }
_, conflict, err := ctx.Notes().Update(n.ID, "stale edit", n.UpdatedAt)
return conflict, err`)
	contains(t, result.Text, "Result: true", "did not save note")
	if got, _ := f.notes.Get(note.ID); got.Markdown != "first edit" {
		t.Fatalf("note = %q; the stale edit must not be written", got.Markdown)
	}
}

func TestProgramCreatesWithTheTwoCallForm(t *testing.T) {
	f := newFixture(t)
	result := f.run(`n, err := ctx.Notes().Create()
if err != nil { return nil, err }
_, _, err = ctx.Notes().Update(n.ID, "two calls", n.UpdatedAt)
return n.ID, err`)
	if result.Failed || len(f.noteIDs()) != 1 {
		t.Fatalf("result = %+v, notes = %v", result, f.noteIDs())
	}
}

func TestProgramComputesWithTheAllowedPackages(t *testing.T) {
	f := newFixture(t)
	result := f.run(`words := strings.Fields("  alpha beta  gamma ")
sort.Sort(sort.Reverse(sort.StringSlice(words)))
raw, err := json.Marshal(map[string]any{"words": words, "n": strconv.Itoa(len(words))})
return string(raw), err`)
	if result.Failed {
		t.Fatalf("failed: %s", result.Text)
	}
	contains(t, result.Text, `gamma`, `beta`, "No Nevermind calls were made.")
}

func TestProgramErrorsReachTheModelWithTheJournal(t *testing.T) {
	f := newFixture(t)
	result := f.run(`_, err := ctx.Notes().CreateWith("kept")
if err != nil { return nil, err }
return nil, fmt.Errorf("the second step failed")`)
	if !result.Failed {
		t.Fatal("a program that returns an error must end failed")
	}
	contains(t, result.Text, "The program returned an error: the second step failed", "created note")
	if !strings.Contains(result.Outcome, "Failed") {
		t.Fatalf("outcome = %q", result.Outcome)
	}
}
