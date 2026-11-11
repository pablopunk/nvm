package compositor

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/hyprland"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

func TestDesiredForBindsThePaletteAndActionsThroughTheCommandLine(t *testing.T) {
	d := DesiredFor("Control+Alt+K", map[string]string{
		shortcuts.CommandQuickNotes: "Alt+N",
		shortcuts.PaletteID:         "Control+Alt+K",
		"app:/opt/My App.desktop":   "Alt+A",
		shortcuts.CommandSnippets:   "",
	}, Identity{Executable: "/opt/Nevermind Next/nvm", Classes: []string{"nvm"}}, true)
	var got []string
	for _, b := range d.Binds {
		got = append(got, b.Accelerator+" => "+b.Command)
	}
	want := []string{
		"Control+Alt+K => '/opt/Nevermind Next/nvm' run palette-toggle",
		"Alt+A => '/opt/Nevermind Next/nvm' run 'app:/opt/My App.desktop'",
		"Alt+N => '/opt/Nevermind Next/nvm' run command:quick-notes",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("binds\n%s", strings.Join(got, "\n"))
	}
	if !d.Appearance.Undecorated || d.Appearance.Classes[0] != "nvm" {
		t.Fatalf("appearance %+v", d.Appearance)
	}
}

func newService(t *testing.T) *Service {
	dir := t.TempDir()
	env := hyprland.Env{Getenv: func(string) string { return "" }, UndoFile: filepath.Join(dir, "undo.json")}
	return New(env, filepath.Join(dir, "state.json"), time.Now)
}

func TestOwnedShortcutsSkipThePortal(t *testing.T) {
	s := newService(t)
	portal := fake.NewHotkeys()
	changes := 0
	keys := s.Hotkeys(portal, func() { changes++ })
	if err := keys.Register("Control+Alt+K", func() {}); err != nil || len(portal.Registered()) != 1 {
		t.Fatalf("not owned: %v %v", err, portal.Registered())
	}
	keys.Unregister("Control+Alt+K")
	if err := s.saveState(State{OwnsShortcuts: true}); err != nil {
		t.Fatal(err)
	}
	if err := keys.Register("Control+Alt+K", func() {}); err != nil || len(portal.Registered()) != 0 {
		t.Fatalf("owned key reached the portal: %v %v", err, portal.Registered())
	}
	keys.Unregister("Control+Alt+K")
	if changes != 4 {
		t.Fatalf("changes %d", changes)
	}
	// Registering the same key again is not a change.
	keys.Register("Alt+N", func() {})
	keys.Register("Alt+N", func() {})
	if changes != 5 {
		t.Fatalf("changes %d", changes)
	}
}

func TestStateSurvivesARestart(t *testing.T) {
	s := newService(t)
	if err := s.SetUndecorated(true); err != nil {
		t.Fatal(err)
	}
	again := New(s.env, s.statePath, time.Now)
	if !again.State().Undecorated {
		t.Fatal("the appearance choice was lost")
	}
	if info, err := os.Stat(s.statePath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("state file: %v %v", info, err)
	}
}

func TestLifecycleWithoutHyprlandFailsTruthfully(t *testing.T) {
	s := newService(t)
	if s.Present() {
		t.Fatal("present without a session")
	}
	if _, err := s.Preview(context.Background(), hyprland.Desired{}); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("preview: %v", err)
	}
	if _, err := s.Remove(context.Background()); err == nil {
		t.Fatal("removed without a session")
	}
	if last := s.Last(); last == nil || last.Err == nil || last.Step != "remove" {
		t.Fatalf("last %+v", last)
	}
}
