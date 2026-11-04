package commands

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

func TestEveryLayoutPresetHasACommand(t *testing.T) {
	commands := All()
	seen := map[geometry.PresetID]bool{}
	for _, command := range commands {
		if command.IsFocus() {
			continue
		}
		seen[command.Preset()] = true
	}
	for _, id := range geometry.PresetIDs() {
		if !seen[id] {
			t.Errorf("preset %q has no command", id)
		}
	}
}

func TestCommandIDsAreUniqueAndPrefixed(t *testing.T) {
	seen := map[string]bool{}
	for _, command := range All() {
		if !strings.HasPrefix(command.ID, idPrefix) {
			t.Errorf("command %q lacks the %q prefix", command.ID, idPrefix)
		}
		if seen[command.ID] {
			t.Errorf("duplicate command ID %q", command.ID)
		}
		seen[command.ID] = true
		if command.Title == "" || command.Icon == "" || len(command.Keywords) == 0 {
			t.Errorf("command %q has no presentation", command.ID)
		}
	}
}

func TestPersistedIDsResolve(t *testing.T) {
	cases := map[string]string{
		"window.left-half":   "left-half",
		"window.restore":     "restore",
		"window.focus-right": "focus-right",
	}
	for id := range cases {
		command, ok := ByID(id)
		if !ok {
			t.Fatalf("ByID(%q) not found", id)
		}
		if command.ID != id {
			t.Errorf("ByID(%q).ID = %q", id, command.ID)
		}
	}
	if command, _ := ByID("window.focus-right"); command.Direction() != geometry.NeighborRight {
		t.Errorf("focus-right direction = %q", command.Direction())
	}
}

func TestUnknownIDIsNotFound(t *testing.T) {
	if _, ok := ByID("window.no-such-layout"); ok {
		t.Fatal("unknown ID resolved")
	}
}

func TestFocusCommandsMatchElectronCopy(t *testing.T) {
	command, _ := ByID("window.focus-up")
	if command.Title != "Focus Window Up" {
		t.Errorf("title = %q", command.Title)
	}
}
