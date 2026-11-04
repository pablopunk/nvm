package app

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TestExtensionRowWithoutAnAuthorOffersRenameAndTweak checks an installed extension the builder did not make: 1.x
// offers Rename and Tweak with AI for any file-backed extension, and Duplicate and Remove action only for generated ones.
func TestExtensionRowWithoutAnAuthorOffersRenameAndTweak(t *testing.T) {
	p := goldenPalette(t)
	actions := p.extensionAuthoringActions("acme.notes", "Notes")
	if got, want := searchActionTitles(actions), []string{renameTitle, tweakWithAITitle}; !slices.Equal(got, want) {
		t.Fatalf("authoring actions = %v, want %v", got, want)
	}
	if actions[0].Disabled != renameUnavailable {
		t.Fatalf("Rename disabled reason = %q, want %q", actions[0].Disabled, renameUnavailable)
	}
	if actions[1].Disabled != "" {
		t.Fatal("Tweak with AI is disabled; it opens the builder chat")
	}
}

// TestBuiltInRowsHaveNoAuthoringActions checks that a built-in extension's row is left as it is.
func TestBuiltInRowsHaveNoAuthoringActions(t *testing.T) {
	p := goldenPalette(t)
	row := search.Item{Title: "Lock Screen", Actions: []search.Action{{Title: "Lock Screen"}}}
	if got := p.withAuthoringActions(row, systemcommands.ID); !slices.Equal(searchActionTitles(got.Actions), []string{"Lock Screen"}) {
		t.Fatalf("built-in row actions = %v", searchActionTitles(got.Actions))
	}
}
