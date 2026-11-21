//go:build rig

package rigtest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	// j18WatchFolder is the folder the acme.triggers fixture watches, under the launch's HOME.
	j18WatchFolder = "nvm-j18-watch"
	// j18Settle is longer than the 800 ms debounce and the trigger sync, so a second run would show up in it.
	j18Settle = 3 * time.Second
	// j18Probe is the spacing of the files written until the watch has started, and j18Look bounds one palette look.
	j18Probe = time.Second
	j18Look  = 2 * time.Second
)

// TestJourneyJ18TriggersFireOncePerChange is J18: a files.changed trigger runs once for a change and once for a burst
// of changes made together, never once per file, and clipboard.changed runs once per copy. The fixture's search row
// shows how many times each trigger ran, and the rig reads it from the palette.
// Not covered by the rig yet: the no-overlap guarantee (the scheduler's unit tests cover it), the interval and wake
// triggers, and app.frontmost.changed.
func TestJourneyJ18TriggersFireOncePerChange(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, "acme.triggers")
	watch := filepath.Join(home, j18WatchFolder)
	must(t, os.MkdirAll(watch, 0o755))
	app := Launch(t, Options{Home: home})

	openExtensionsView(t, app)
	enableThroughReview(t, app, "trigger", "Trigger fixture")
	hidePalette(t, app)

	writeUntilCounted(t, app, watch, "files 1, clipboard 0")
	time.Sleep(j18Settle)
	expectTriggerCounts(t, app, "files 1, clipboard 0")
	shoot(t, app, "j18-1-one-change-one-run")

	for i := range 3 {
		must(t, os.WriteFile(filepath.Join(watch, fmt.Sprintf("burst-%d.txt", i)), []byte("burst"), 0o644))
	}
	waitForTriggerCounts(t, app, "files 2, clipboard 0")
	time.Sleep(j18Settle)
	expectTriggerCounts(t, app, "files 2, clipboard 0")
	shoot(t, app, "j18-2-burst-debounced-to-one-run")

	must(t, app.ClipboardSet("j18 clipboard copy"))
	waitForTriggerCounts(t, app, "files 2, clipboard 1")
	time.Sleep(j18Settle)
	expectTriggerCounts(t, app, "files 2, clipboard 1")
	shoot(t, app, "j18-3-copy-one-run")

	clearSearch(t, app)
	hidePalette(t, app)
	if app.isExited() {
		t.Fatalf("the app exited during J18; see %s", app.logPath)
	}
}

// writeUntilCounted writes one file after another until the counts show want. The watch starts a moment after the
// extension is enabled, so a file written before then is not a change the trigger sees, and the next write retries.
func writeUntilCounted(t *testing.T, app *App, watch, want string) {
	t.Helper()
	deadline := time.Now().Add(clipWait)
	for i := 0; ; i++ {
		must(t, os.WriteFile(filepath.Join(watch, fmt.Sprintf("probe-%d.txt", i)), []byte("probe"), 0o644))
		time.Sleep(j18Probe)
		if triggerCountsShown(t, app, want, j18Look) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the palette never showed %q for the files trigger", want)
		}
	}
}

// waitForTriggerCounts looks at the counts until they show want, within the clipboard wait.
func waitForTriggerCounts(t *testing.T, app *App, want string) {
	t.Helper()
	deadline := time.Now().Add(clipWait)
	for !triggerCountsShown(t, app, want, j18Look) {
		if time.Now().After(deadline) {
			t.Fatalf("the palette never showed %q", want)
		}
	}
}

// expectTriggerCounts fails when the counts are not want: a second run would show as a higher count.
func expectTriggerCounts(t *testing.T, app *App, want string) {
	t.Helper()
	if !triggerCountsShown(t, app, want, elementWait) {
		t.Fatalf("the trigger counts are not %q", want)
	}
}

// triggerCountsShown opens the palette if it is hidden, searches afresh for the counts row and reports whether a row
// within the timeout shows want in its title.
func triggerCountsShown(t *testing.T, app *App, want string, timeout time.Duration) bool {
	t.Helper()
	visible, err := app.PaletteVisible()
	must(t, err)
	if !visible {
		must(t, app.SummonPalette())
	}
	clearSearch(t, app)
	must(t, app.Type("trigger"))
	_, err = app.WaitFor(want, timeout)
	return err == nil
}
