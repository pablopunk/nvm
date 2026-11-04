//go:build linux

package real

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// fakeXdotool installs an xdotool on the PATH that answers with the given window and process, and appends each
// windowactivate call to a log, so the adapter is tested without an X server.
func fakeXdotool(t *testing.T, activeWindow, owner string) (activations func() string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "activations.log")
	script := `#!/bin/sh
case "$1" in
  getactivewindow) [ -n "` + activeWindow + `" ] && echo "` + activeWindow + `" || exit 1 ;;
  getwindowpid) [ -n "` + owner + `" ] && echo "` + owner + `" || exit 1 ;;
  windowactivate) echo "$*" >> "` + log + `" ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "xdotool"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() string {
		data, _ := os.ReadFile(log)
		return strings.TrimSpace(string(data))
	}
}

func TestActiveReadsTheFocusedWindowAndItsOwner(t *testing.T) {
	fakeXdotool(t, "41943045", "812")
	got, err := focusX11{}.Active()
	if err != nil || got != (platform.FocusTarget{Window: "41943045", PID: 812}) {
		t.Fatalf("Active = %+v, %v", got, err)
	}
}

func TestActiveWithNoFocusedWindowIsNotFound(t *testing.T) {
	fakeXdotool(t, "", "")
	if _, err := (focusX11{}).Active(); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Active error = %v, want ErrNotFound", err)
	}
}

func TestRestoreActivatesTheWindowWhenItsOwnerIsUnchanged(t *testing.T) {
	activations := fakeXdotool(t, "41943045", "812")
	if err := (focusX11{}).Restore(platform.FocusTarget{Window: "41943045", PID: 812}); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := activations(); got != "windowactivate --sync 41943045" {
		t.Fatalf("activations = %q", got)
	}
}

func TestRestoreRefusesAWindowThatChangedOwner(t *testing.T) {
	activations := fakeXdotool(t, "41943045", "999")
	err := (focusX11{}).Restore(platform.FocusTarget{Window: "41943045", PID: 812})
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Restore error = %v, want ErrNotFound", err)
	}
	if got := activations(); got != "" {
		t.Fatalf("activated %q for a window another process owns", got)
	}
}
