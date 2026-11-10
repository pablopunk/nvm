package api

import (
	"strings"
	"testing"
)

// TestManifestRejectsAnEmptyCommandTitle checks that a command rename to a blank title is a load problem, not a silent
// unnamed command.
func TestManifestRejectsAnEmptyCommandTitle(t *testing.T) {
	m := Manifest{APIVersion: "1.0", Identity: Identity{ID: "acme.notes", Title: "Notes"},
		CommandTitles: map[string]string{"new": "  "}}
	err := m.Validate()
	if err == nil || !strings.Contains(err.Error(), "commandTitles.new") {
		t.Fatalf("err = %v", err)
	}
}
