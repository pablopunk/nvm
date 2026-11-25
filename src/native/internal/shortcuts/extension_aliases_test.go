package shortcuts

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type aliasEditorFixture struct {
	aliases *Aliases
	editor  ExtensionAliasEditor
	changes int
}

func newAliasEditorFixture(t *testing.T) *aliasEditorFixture {
	t.Helper()
	managed := newManagerFixture(t, "darwin", DefaultDocument("Alt+Shift+Space"), nil)
	managed.manager.Start()
	f := &aliasEditorFixture{aliases: NewAliases(NewMemoryAliases())}
	table := NewExtensionTable(managed.manager, func() Labeler { return Labeler{GOOS: "darwin"} }, f.aliases)
	f.editor = NewExtensionAliasEditor(table, func() { f.changes++ })
	return f
}

func requireInvalid(t *testing.T, err error, mentions string) {
	t.Helper()
	if !errors.Is(err, api.ErrInvalidInput) || !strings.Contains(err.Error(), mentions) {
		t.Fatalf("err = %v; want invalid input mentioning %q", err, mentions)
	}
}

func TestPlanningAnAliasNamesTheCommandAndSavesNothing(t *testing.T) {
	f := newAliasEditorFixture(t)
	change, err := f.editor.PlanAlias("command:clipboard", "  clip  ")
	if err != nil || change.Title != "Clipboard History" || change.Alias != "clip" {
		t.Fatalf("change = %+v, err = %v", change, err)
	}
	if len(f.aliases.All()) != 0 || f.changes != 0 {
		t.Fatalf("a plan saved aliases: %v", f.aliases.All())
	}
}

func TestAddingAnAliasSavesItAndReportsTheChange(t *testing.T) {
	f := newAliasEditorFixture(t)
	if err := f.editor.AddAlias("command:clipboard", "clip"); err != nil {
		t.Fatal(err)
	}
	if got := f.aliases.For("command:clipboard"); len(got) != 1 || got[0] != "clip" || f.changes != 1 {
		t.Fatalf("aliases = %v, changes = %d", got, f.changes)
	}
}

func TestAnAliasIsRefusedForAnUnknownCommandABadPhraseOrATakenOne(t *testing.T) {
	f := newAliasEditorFixture(t)
	if err := f.aliases.Add("command:notes", "Jot"); err != nil {
		t.Fatal(err)
	}
	requireInvalid(t, f.editor.AddAlias("command:missing", "x"), "no command")
	requireInvalid(t, f.editor.AddAlias("command:clipboard", "   "), "at least one letter")
	requireInvalid(t, f.editor.AddAlias("command:clipboard", strings.Repeat("a", MaxAliasRunes+1)), "at most")
	requireInvalid(t, f.editor.AddAlias("command:clipboard", "two\nlines"), "one line")
	requireInvalid(t, f.editor.AddAlias("command:clipboard", "jot"), "already an alias of Floating Notes")
	if got := f.aliases.For("command:clipboard"); len(got) != 0 || f.changes != 0 {
		t.Fatalf("a refused alias was saved: %v", got)
	}
}

func TestRemovingAnAliasFindsItsCommand(t *testing.T) {
	f := newAliasEditorFixture(t)
	if err := f.aliases.Add("command:notes", "Jot"); err != nil {
		t.Fatal(err)
	}
	change, err := f.editor.PlanAlias("", "jot")
	if err != nil || change.ActionID != "command:notes" || change.Title != "Floating Notes" {
		t.Fatalf("change = %+v, err = %v", change, err)
	}
	if err := f.editor.RemoveAlias("jot"); err != nil {
		t.Fatal(err)
	}
	if len(f.aliases.All()) != 0 || f.changes != 1 {
		t.Fatalf("aliases = %v, changes = %d", f.aliases.All(), f.changes)
	}
	requireInvalid(t, f.editor.RemoveAlias("jot"), "no command has the alias")
}

// The check that an alias is free runs again as it is saved, so a phrase taken since the plan is not saved twice.
func TestClaimRefusesAnAliasAnotherActionHas(t *testing.T) {
	aliases := NewAliases(NewMemoryAliases())
	if owner, err := aliases.Claim("command:notes", "jot"); owner != "" || err != nil {
		t.Fatalf("owner = %q, err = %v", owner, err)
	}
	if owner, err := aliases.Claim("command:clipboard", "JOT"); owner != "command:notes" || err != nil {
		t.Fatalf("owner = %q, err = %v", owner, err)
	}
	if got := aliases.For("command:clipboard"); len(got) != 0 {
		t.Fatalf("a taken alias was saved: %v", got)
	}
}
