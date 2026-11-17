package app

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TestExtensionRowWithoutAnAuthorOffersRenameTweakAndRemove checks an installed extension the builder did not make: it
// offers Rename, Tweak with AI and Remove extension for any file-backed extension, and Duplicate and Submit as PR only for
// generated ones. Each one that is listed runs.
const tweakWithAITitle = "Tweak with AI"

func TestExtensionRowWithoutAnAuthorOffersRenameTweakAndRemove(t *testing.T) {
	p := goldenPalette(t)
	actions := p.extensionAuthoringActions("acme.notes", "", "Notes")
	if got, want := searchActionTitles(actions), []string{renameTitle, tweakWithAITitle, removeActionTitle}; !slices.Equal(got, want) {
		t.Fatalf("authoring actions = %v, want %v", got, want)
	}
	for _, a := range actions {
		if a.Disabled != "" {
			t.Fatalf("%s is disabled (%q); 1.x shows it enabled", a.Title, a.Disabled)
		}
	}
}

// TestBuiltInRowsHaveNoAuthoringActions checks that a built-in extension's row is left as it is.
func TestBuiltInRowsHaveNoAuthoringActions(t *testing.T) {
	p := goldenPalette(t)
	row := search.Item{Title: "Lock Screen", Actions: []search.Action{{Title: "Lock Screen"}}}
	if got := p.withAuthoringActions(row, systemcommands.ID, ""); !slices.Equal(searchActionTitles(got.Actions), []string{"Lock Screen"}) {
		t.Fatalf("built-in row actions = %v", searchActionTitles(got.Actions))
	}
}
