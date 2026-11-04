//go:build rig

package rigtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fix journey runs Fix Selected Text with AI in a real text editor (mousepad, in the rig image): the selection is
// read through the X11 copy gesture, the corrected text replaces it with a paste, the clipboard the user had comes
// back, and Undo Fix from the palette puts the original text back. The editor saves to a file the test reads, so the
// text the editor holds is what the test asserts.

const (
	editorFileName  = "fix.txt"
	editorInput     = "this are selected text in the editor"
	editorCorrected = "This is selected text in the editor."
	editorClipboard = "rig clipboard kept by the editor fix"
	editorWait      = 20 * time.Second
	editorSaveWait  = 10 * time.Second
	editorWindow    = "fix\\.txt"
)

// TestJourneyAIFixInTextEditor fixes a sentence in mousepad, checks the clipboard, and undoes the fix.
func TestJourneyAIFixInTextEditor(t *testing.T) {
	fake := StartFakeBackend(t)
	fake.SetReply(editorCorrected)
	app := Launch(t, Options{Backend: fake.URL})
	signInThroughAccount(t, app, fake)
	hidePalette(t, app)
	must(t, app.ClipboardSet(editorClipboard))

	file := filepath.Join(t.TempDir(), editorFileName)
	writeFile(t, file, editorInput+"\n", 0o644)
	display := os.Getenv("DISPLAY")
	startEditor(t, file)
	window := waitForWindowMatching(t, display, editorWindow)
	focusEditor(t, display, window)
	must(t, editorKey(display, "ctrl+a"))

	must(t, app.SummonPalette())
	must(t, app.Type("Fix Selected Text"))
	_, err := app.WaitFor("Fix Selected Text with AI", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	focusEditor(t, display, window)
	saveEditor(t, display, window)
	waitFileHolds(t, file, editorCorrected)
	waitClipboardEquals(t, app, editorClipboard)
	shoot(t, app, "ai-fix-editor-1-fixed")

	must(t, app.SummonPalette())
	must(t, app.Type("Undo Fix"))
	_, err = app.WaitFor("Undo Fix", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	focusEditor(t, display, window)
	saveEditor(t, display, window)
	waitFileHolds(t, file, strings.TrimSpace(editorInput))
	waitClipboardEquals(t, app, editorClipboard)
	shoot(t, app, "ai-fix-editor-2-undone")
}

// startEditor opens file in mousepad and stops it when the test ends.
func startEditor(t *testing.T, file string) {
	t.Helper()
	cmd := exec.Command("mousepad", "--disable-server", file)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mousepad: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
}

// waitForWindowMatching waits for a window whose name matches pattern, a regular expression for xdotool.
func waitForWindowMatching(t *testing.T, display, pattern string) string {
	t.Helper()
	deadline := time.Now().Add(editorWait)
	for {
		out, err := xdotoolOn(display, "search", "--name", pattern)
		if err == nil {
			if ids := strings.Fields(out); len(ids) > 0 {
				return ids[len(ids)-1]
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no window matches %q", pattern)
		}
		time.Sleep(pollInterval)
	}
}

// focusEditor gives the keyboard to the editor window.
func focusEditor(t *testing.T, display, window string) {
	t.Helper()
	must(t, activateWindow(display, window))
}

// editorKey sends one key chord to the focused window on display.
func editorKey(display, chord string) error {
	_, err := xdotoolOn(display, "key", "--clearmodifiers", chord)
	return err
}

// saveEditor saves the focused editor to its file.
func saveEditor(t *testing.T, display, window string) {
	t.Helper()
	focusEditor(t, display, window)
	must(t, editorKey(display, "ctrl+s"))
}

// waitFileHolds polls file until it holds want, failing after editorSaveWait.
func waitFileHolds(t *testing.T, file, want string) {
	t.Helper()
	deadline := time.Now().Add(editorSaveWait)
	for {
		data, err := os.ReadFile(file)
		if err == nil && strings.Contains(string(data), want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s does not hold %q; it holds %q", file, want, string(data))
		}
		time.Sleep(pollInterval)
	}
}
