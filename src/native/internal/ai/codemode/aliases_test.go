package codemode

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// fakeAliasEditor is the host's alias list over the fake shortcut table's commands: it names commands from the table and
// refuses an unknown command, a blank alias and one another command has, as the host does.
type fakeAliasEditor struct {
	table *fakeShortcutTable
	mu    sync.Mutex
	owner map[string]string
}

func newFakeAliasEditor(table *fakeShortcutTable) *fakeAliasEditor {
	return &fakeAliasEditor{table: table, owner: map[string]string{"jot": "notes"}}
}

func (f *fakeAliasEditor) PlanAlias(actionID, alias string) (protocol.AliasChange, error) {
	alias = strings.TrimSpace(alias)
	change := protocol.AliasChange{ActionID: actionID, Alias: alias}
	if alias == "" {
		return change, fmt.Errorf("an alias needs at least one letter: %w", api.ErrInvalidInput)
	}
	f.mu.Lock()
	owner, taken := f.owner[alias]
	f.mu.Unlock()
	if actionID == "" {
		if !taken {
			return change, fmt.Errorf("no command has the alias %q: %w", alias, api.ErrInvalidInput)
		}
		change.ActionID, change.Title = owner, f.table.record(owner).Title
		return change, nil
	}
	change.Title = f.table.record(actionID).Title
	if change.Title == "" {
		return change, fmt.Errorf("no command has the action id %q: %w", actionID, api.ErrInvalidInput)
	}
	if taken {
		return change, fmt.Errorf("“%s” is already an alias of %s: %w", alias, f.table.record(owner).Title, api.ErrInvalidInput)
	}
	return change, nil
}

func (f *fakeAliasEditor) AddAlias(actionID, alias string) error {
	if _, err := f.PlanAlias(actionID, alias); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.owner[alias] = actionID
	return nil
}

func (f *fakeAliasEditor) RemoveAlias(alias string) error {
	if _, err := f.PlanAlias("", alias); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.owner, alias)
	return nil
}

func (f *fakeAliasEditor) ownerOf(alias string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.owner[alias]
}

func withAliases(table *fakeShortcutTable, aliases *fakeAliasEditor) fixtureOption {
	return func(opts *Options) {
		withShortcuts(table)(opts)
		opts.AliasEditor = aliases
	}
}

func aliasFixture(t *testing.T) (*fixture, *fakeAliasEditor) {
	table := newFakeShortcutTable()
	aliases := newFakeAliasEditor(table)
	return newFixture(t, withAliases(table, aliases)), aliases
}

// Adding an alias asks with the command's title from the host, saves the alias as the host cleaned it, and journals it.
func TestAddingAnAliasAsksWithTheCommandTitleAndRunsWhenAllowed(t *testing.T) {
	f, aliases := aliasFixture(t)
	f.steps.decide = allow
	result := f.run(`return nil, ctx.Shortcuts().AddAlias("clipboard", "  clip  ")`)
	if result.Failed {
		t.Fatalf("failed: %s", result.Text)
	}
	if got := aliases.ownerOf("clip"); got != "clipboard" {
		t.Fatalf("clip belongs to %q", got)
	}
	row := lastRow(t, f, "Adding an alias")
	if !row.asked || row.detail != "Add alias “clip” for “Clipboard History”" {
		t.Fatalf("row = %+v", row)
	}
	contains(t, result.Text, "added alias “clip” for “Clipboard History” (the user allowed it)")
}

func TestADeclinedAliasChangeSavesNothing(t *testing.T) {
	f, aliases := aliasFixture(t)
	f.steps.decide = decline
	result := f.run(`return nil, ctx.Shortcuts().RemoveAlias("jot")`)
	if !result.Failed || aliases.ownerOf("jot") != "notes" {
		t.Fatalf("result = %+v, jot belongs to %q", result, aliases.ownerOf("jot"))
	}
	if row := lastRow(t, f, "Removing an alias"); !row.asked || row.detail != "Remove alias “jot” from “Search Notes”" {
		t.Fatalf("row = %+v", row)
	}
	contains(t, result.Text, "remove alias “jot” from “Search Notes”: the user declined")
}

// An alias change the host refuses while preparing it fails with the host's reason, and the user is never asked.
func TestARefusedAliasFailsWithoutAsking(t *testing.T) {
	cases := map[string]struct{ body, reason string }{
		"unknown command": {`return nil, ctx.Shortcuts().AddAlias("missing", "x")`, "no command has the action id"},
		"blank alias":     {`return nil, ctx.Shortcuts().AddAlias("clipboard", "  ")`, "at least one letter"},
		"taken alias":     {`return nil, ctx.Shortcuts().AddAlias("clipboard", "jot")`, "already an alias of Search Notes"},
		"unknown alias":   {`return nil, ctx.Shortcuts().RemoveAlias("nope")`, "no command has the alias"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f, aliases := aliasFixture(t)
			f.steps.decide = allow
			result := f.run(c.body)
			if !result.Failed || !strings.Contains(result.Text, c.reason) {
				t.Fatalf("result = %+v", result)
			}
			for _, row := range f.steps.snapshot() {
				if row.asked {
					t.Fatalf("a refused alias change was put to the user: %+v", row)
				}
			}
			if aliases.ownerOf("jot") != "notes" || aliases.ownerOf("x") != "" {
				t.Fatalf("a refused change saved an alias: %+v", aliases.owner)
			}
		})
	}
}

func TestReadingAliasesDoesNotAsk(t *testing.T) {
	f, _ := aliasFixture(t)
	f.run(`return ctx.Shortcuts().Aliases("c"), nil`)
	for _, row := range f.steps.snapshot() {
		if row.asked {
			t.Fatalf("a read asked: %+v", row)
		}
	}
}
