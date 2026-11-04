package aicommands

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
)

// TestEnterReplacesWhenTheResultCanBeReplaced: a proofread's Enter hands the shown text to Replace, and never copies.
func TestEnterReplacesWhenTheResultCanBeReplaced(t *testing.T) {
	var replaced, copied []string
	view := ResultView(commands.Result{
		Title:   "Fixed Text",
		Text:    func() string { return "This is selected text." },
		Replace: func(text string) { replaced = append(replaced, text) },
		Copy:    func(text string) { copied = append(copied, text) },
	})
	view.OnSubmit("")
	if len(replaced) != 1 || replaced[0] != "This is selected text." {
		t.Fatalf("Replace got %q, want the corrected text once", replaced)
	}
	if len(copied) != 0 {
		t.Fatalf("Enter copied %q although the result replaces", copied)
	}
}

// TestEnterCopiesWhenTheResultCannotBeReplaced: a classification has no selection to replace, so Enter copies it.
func TestEnterCopiesWhenTheResultCannotBeReplaced(t *testing.T) {
	var copied []string
	view := ResultView(commands.Result{
		Title: "Image Classification",
		Text:  func() string { return "screenshot" },
		Copy:  func(text string) { copied = append(copied, text) },
	})
	if got := view.Hint; got != copyHint {
		t.Fatalf("hint = %q, want the copy hint", got)
	}
	view.OnSubmit("")
	if len(copied) != 1 || copied[0] != "screenshot" {
		t.Fatalf("Copy got %q, want the category once", copied)
	}
}

// TestHeightGrowsOnlyForLongFailures: a failure's copy wraps and the panel grows to show all of it; the other states
// keep the base height.
func TestHeightGrowsOnlyForLongFailures(t *testing.T) {
	if got := Height(commands.Status{Subtitle: strings.Repeat("x", 500), Tone: commands.ToneLoading}); got != BaseHeight {
		t.Fatalf("loading height = %d, want %d", got, BaseHeight)
	}
	short := Height(commands.Status{Subtitle: "Sign in to use AI commands.", Tone: commands.ToneError})
	long := Height(commands.Status{Subtitle: strings.Repeat("word ", 40), Tone: commands.ToneError})
	if short != BaseHeight || long <= BaseHeight {
		t.Fatalf("failure heights short=%d long=%d; want the base height, then a taller panel", short, long)
	}
}
