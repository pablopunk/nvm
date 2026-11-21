package manager

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestCommandTitlesFromTheManifestOverrideTheCodeTitle renames one command in the manifest's titles and leaves the other
// command's code title as it is.
func TestCommandTitlesFromTheManifestOverrideTheCodeTitle(t *testing.T) {
	commands := []api.Command{{ID: "new", Title: "New Note"}, {ID: "open", Title: "Open Note"}}
	applyCommandTitles(commands, map[string]string{"new": "Jot down"})
	if commands[0].Title != "Jot down" || commands[1].Title != "Open Note" {
		t.Fatalf("titles = %q, %q", commands[0].Title, commands[1].Title)
	}
}
