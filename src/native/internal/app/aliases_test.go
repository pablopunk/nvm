package app

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// setAliasRow returns the Set alias action of a row: it is the last of the row's menu.
func setAliasRow(t *testing.T, row search.Item) search.Action {
	t.Helper()
	if got := row.Actions[len(row.Actions)-1].Title; got != setAliasTitle {
		t.Fatalf("last action = %q, want %q", got, setAliasTitle)
	}
	return row.Actions[len(row.Actions)-1]
}

// TestSetAliasOpensTheAliasViewOfTheRow checks that Set alias pushes the alias view, titled with the row, as 1.x does.
func TestSetAliasOpensTheAliasViewOfTheRow(t *testing.T) {
	p := goldenPalette(t)
	setAliasRow(t, openClipboardCommandRow(t, p)).Run(p)
	if got := p.Top().Title; got != "Set alias" {
		t.Fatalf("top view = %q, want the alias view", got)
	}
	if got := p.Top().Placeholder; got != "Alias for “"+clipboardCommandTitle+"”" {
		t.Fatalf("placeholder = %q", got)
	}
}

// TestAliasSavedInTheViewFindsTheRowInTheRootSearch saves a phrase that matches no title, then searches the root
// with it: the row is found, and removing the alias makes it disappear again.
func TestAliasSavedInTheViewFindsTheRowInTheRootSearch(t *testing.T) {
	const phrase = "zq vault"
	p := shownPalette(t)
	setAliasRow(t, openClipboardCommandRow(t, p)).Run(p)
	goldenQuery(p, phrase)
	saveRow := p.Results()[0]
	if saveRow.Title != "Save alias “"+phrase+"”" {
		t.Fatalf("first alias row = %q, want the save row", saveRow.Title)
	}
	saveRow.Actions[0].Run(p)
	if got := p.aliasStore().For(shortcuts.CommandClipboardHistory); !slices.Equal(got, []string{phrase}) {
		t.Fatalf("saved aliases = %v, want [%s]", got, phrase)
	}
	p.Pop()
	goldenQuery(p, phrase)
	if _, row := rootRowTitled(t, p, clipboardCommandTitle); row.Title != clipboardCommandTitle {
		t.Fatal("the alias did not find the command in the root search")
	}
	p.Pop()
	goldenQuery(p, "")
	setAliasRow(t, openClipboardCommandRow(t, p)).Run(p)
	goldenQuery(p, phrase)
	_, removeRow := rootRowTitled(t, p, phrase)
	removeRow.Actions[0].Run(p)
	if got := p.aliasStore().For(shortcuts.CommandClipboardHistory); len(got) != 0 {
		t.Fatalf("aliases after remove = %v, want none", got)
	}
}

// TestAliasDoesNotMatchAnotherRowsTitle checks that an alias of one command leaves the others as they are.
func TestAliasDoesNotMatchAnotherRowsTitle(t *testing.T) {
	p := goldenPalette(t)
	if err := p.aliasStore().Add(shortcuts.CommandClipboardHistory, "zq vault"); err != nil {
		t.Fatal(err)
	}
	goldenQuery(p, "zq vault")
	for _, row := range p.Results() {
		if row.Title == "Emoji & Symbols" {
			t.Fatal("an alias of Clipboard History found Emoji & Symbols")
		}
	}
}

// TestAliasFindsAnExtensionCommandRow saves an alias of an extension command and finds the command by it: its title does not
// match the phrase, so only the alias can find it.
func TestAliasFindsAnExtensionCommandRow(t *testing.T) {
	p := goldenPalette(t)
	if err := p.aliasStore().Add("extension:acme.notes:new", "zq jot"); err != nil {
		t.Fatal(err)
	}
	row := extensionCommandRow(t, p, "zq jot")
	if row.Title != "New Note" {
		t.Fatalf("row = %q, want New Note", row.Title)
	}
}
