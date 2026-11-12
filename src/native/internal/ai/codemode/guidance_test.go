package codemode

import (
	"os"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/builder/apiref"
)

// A change to the api that go generate did not carry here fails this test, so the model never reads a stale call.
func TestAPISubsetMatchesTheAPI(t *testing.T) {
	want, err := apiref.RenderSubset(os.DirFS("../../ext/api"), []string{"Notes"})
	if err != nil {
		t.Fatal(err)
	}
	if apiSubset != want {
		t.Fatal("reference_gen.go is stale: run go generate in internal/ai/codemode")
	}
}

// The guidance rides in every request of every conversation, so it has a size budget, not just a style.
func TestGuidanceStaysWithinItsBudget(t *testing.T) {
	text := guidance()
	t.Logf("run_program guidance: %d bytes (budget %d)", len(text), MaxGuidanceBytes)
	if len(text) > MaxGuidanceBytes {
		t.Fatalf("the guidance is %d bytes; the budget is %d", len(text), MaxGuidanceBytes)
	}
}

func TestGuidanceNamesWhatAProgramMayUseAndWhatItCannot(t *testing.T) {
	contains(t, guidance(), "ctx.Notes() Notes", "Search(query string, limit int)", "CreateWith(markdown string)", "strings", "json",
		"no files, network, os, goroutines, channels or generics", "asks the user before it deletes")
}

// exampleBodies splits the guidance's examples into title and program.
func exampleBodies() map[string]string {
	out := map[string]string{}
	for _, section := range strings.Split(strings.TrimSpace(examples), "\n\n") {
		title, body, _ := strings.Cut(section, ":\n")
		out[title] = body
	}
	return out
}

// Each example in the guidance runs against a real notes store and does what its title says.
func TestGuidanceExamplesRun(t *testing.T) {
	bodies := exampleBodies()
	if len(bodies) != 3 {
		t.Fatalf("found %d examples", len(bodies))
	}
	t.Run("create", func(t *testing.T) {
		f := newFixture(t)
		if result := f.run(bodies["Create"]); result.Failed || len(f.noteIDs()) != 1 {
			t.Fatalf("result = %+v", result)
		}
	})
	t.Run("append", func(t *testing.T) {
		f := newFixture(t)
		note := f.note("# Shopping\n- eggs")
		if result := f.run(bodies["Append to a note found by words"]); result.Failed {
			t.Fatalf("result = %+v", result)
		}
		if got, _ := f.notes.Get(note.ID); got.Markdown != "# Shopping\n- eggs\n- milk" {
			t.Fatalf("note = %q", got.Markdown)
		}
	})
	t.Run("delete", func(t *testing.T) {
		f := newFixture(t)
		f.note("# Old ideas")
		f.steps.decide = allow
		if result := f.run(bodies["Delete (asks the user)"]); result.Failed || len(f.noteIDs()) != 0 {
			t.Fatalf("result = %+v, notes %v", result, f.noteIDs())
		}
	})
}
