package real

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// observeTimeout bounds how long a test waits for an event. It is a guard against a missed event, not a sleep: a
// working watch reports within the debounce window.
const observeTimeout = 2 * time.Second

// TestWatchObservesCreateRenameAndDelete runs the real watch on a temporary folder and checks that a file created,
// renamed and deleted under it is reported within two seconds each time. It runs on Linux only: the inotify path is
// the one this test pins down.
func TestWatchObservesCreateRenameAndDelete(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the real watch test pins the Linux backend")
	}
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := NewPorts().FS.Watch(ctx, root)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	created := filepath.Join(root, "created.txt")
	if err := os.WriteFile(created, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, events, "created.txt", func(ev platform.Event) bool {
		return strings.HasSuffix(ev.Path, "created.txt")
	})

	renamed := filepath.Join(root, "renamed.txt")
	if err := os.Rename(created, renamed); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, events, "renamed.txt", func(ev platform.Event) bool {
		return strings.HasSuffix(ev.Path, "renamed.txt") || strings.HasSuffix(ev.OldPath, "created.txt")
	})

	if err := os.Remove(renamed); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, events, "removed", func(ev platform.Event) bool {
		return strings.HasSuffix(ev.Path, "renamed.txt") || ev.Op == platform.EventRemove
	})
}

// awaitEvent waits for an event that matches want, failing the test if none arrives in time or the channel closes.
// Events for other paths are skipped.
func awaitEvent(t *testing.T, events <-chan platform.Event, label string, want func(platform.Event) bool) {
	t.Helper()
	deadline := time.After(observeTimeout)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatalf("watch closed before reporting %s", label)
			}
			if want(ev) {
				return
			}
		case <-deadline:
			t.Fatalf("no event for %s within %v", label, observeTimeout)
		}
	}
}
