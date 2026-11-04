package windows

import (
	"context"
	"testing"
)

func TestShortcutActionsAreTheLayoutsAndFocusMovesOnly(t *testing.T) {
	ids := map[string]bool{}
	for _, action := range ShortcutActions() {
		ids[action.ID] = true
	}
	for _, id := range []string{"window.left-half", "window.restore", "window.focus-up"} {
		if !ids[id] {
			t.Errorf("%s is not bindable", id)
		}
	}
	for _, id := range []string{switchEntryID, settingsEntryID, moveEntryID, resizeEntryID} {
		if ids[id] {
			t.Errorf("%s opens the palette and must not be bindable", id)
		}
	}
}

// A shortcut acts on the app in front when its key was pressed, and never takes the keyboard: the palette stays
// closed and the front app stays in front.
func TestShortcutMovesTheFrontWindowAndDoesNotTakeFocus(t *testing.T) {
	d := newDesk(t)
	if !RunShortcut(d.deps, "window.left-half") {
		t.Fatal("left half is a window command, so the shortcut was not run")
	}
	window, _ := d.windows.Window(d.key)
	if window.Frame.X != 8 {
		t.Errorf("front window x = %v, want the outer gap 8", window.Frame.X)
	}
	front, found, _ := d.windows.Frontmost(context.Background())
	if !found || front.PID != targetPID {
		t.Errorf("front app = %+v, want the target to stay in front", front)
	}
}

func TestShortcutWithNoFrontAppSaysWhy(t *testing.T) {
	d := newEmptyDesk(t)
	if !RunShortcut(d.deps, "window.maximize") {
		t.Fatal("maximize is a window command")
	}
	if note := d.awaitNote(t); note[1] != "No frontmost app was available when the shortcut ran." {
		t.Errorf("note = %q", note)
	}
}

func TestRunShortcutLeavesOtherActionsToTheApp(t *testing.T) {
	d := newDesk(t)
	if RunShortcut(d.deps, "command:Clipboard History") {
		t.Error("a palette command was claimed by the window shortcuts")
	}
}
