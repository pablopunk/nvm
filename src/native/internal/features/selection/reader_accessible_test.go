package selection

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func clipboardText(t *testing.T, r readerRig) string {
	t.Helper()
	text, _ := r.set.Clipboard.ReadText()
	return text
}

// TestReadPrefersTheAccessibleSelection: an app that exposes its selection is read without a keystroke, and the
// clipboard is never touched.
func TestReadPrefersTheAccessibleSelection(t *testing.T) {
	r := newReaderRig(t, nil)
	_ = r.set.Clipboard.WriteText("keep me")
	r.set.SelectedText.SelectAccessible("from accessibility")
	r.set.SelectedText.Select("from the copy")

	got, err := r.reader.Read(fixSource)
	if err != nil || got != "from accessibility" {
		t.Fatalf("Read = %q, %v; want the accessible text", got, err)
	}
	if reqs := r.set.SelectedText.Requests(); len(reqs) != 0 {
		t.Fatalf("copy gestures = %v, want none when accessibility answered", reqs)
	}
	if got := clipboardText(t, r); got != "keep me" {
		t.Fatalf("clipboard = %q, want it untouched", got)
	}
}

// TestReadFallsBackToTheCopyWhenAccessibilityIsEmpty is the Slack case: the app does not expose its selection.
func TestReadFallsBackToTheCopyWhenAccessibilityIsEmpty(t *testing.T) {
	r := newReaderRig(t, nil)
	_ = r.set.Clipboard.WriteText("keep me")
	r.set.SelectedText.Select("typed in Slack")

	got, err := r.reader.Read(fixSource)
	if err != nil || got != "typed in Slack" {
		t.Fatalf("Read = %q, %v; want the copied text", got, err)
	}
	if r.set.SelectedText.AccessibleReads() != 1 {
		t.Fatalf("accessibility reads = %d, want one before the copy", r.set.SelectedText.AccessibleReads())
	}
	if reqs := r.set.SelectedText.Requests(); len(reqs) != 1 {
		t.Fatalf("copy gestures = %v, want one", reqs)
	}
	if got := clipboardText(t, r); got != "keep me" {
		t.Fatalf("clipboard = %q, want the previous text back", got)
	}
}

// TestReadFallsBackToTheCopyWhenAccessibilityFails: an accessibility error is not a failed read.
func TestReadFallsBackToTheCopyWhenAccessibilityFails(t *testing.T) {
	r := newReaderRig(t, nil)
	r.set.SelectedText.FailNext(fake.OpReadAccessibleSelection, platform.ErrDenied)
	r.set.SelectedText.Select("copied")

	if got, err := r.reader.Read(fixSource); err != nil || got != "copied" {
		t.Fatalf("Read = %q, %v; want the copied text", got, err)
	}
}

// TestReadWithHeldModifiersFailsAndRestoresTheClipboard: the shortcut's keys never came up, so no copy was sent.
func TestReadWithHeldModifiersFailsAndRestoresTheClipboard(t *testing.T) {
	r := newReaderRig(t, nil)
	_ = r.set.Clipboard.WriteText("before")
	r.set.SelectedText.Select("text")
	r.set.SelectedText.HoldModifiers(true)

	if _, err := r.reader.Read(fixSource); !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("Read error = %v, want ErrConflict", err)
	}
	if got := clipboardText(t, r); got != "before" {
		t.Fatalf("clipboard = %q, want the previous text back", got)
	}
}

// TestReadRestoresAnEmptyClipboard: nothing on the clipboard before means nothing after, in every path.
func TestReadRestoresAnEmptyClipboard(t *testing.T) {
	r := newReaderRig(t, nil)
	r.set.SelectedText.Select("text")
	if _, err := r.reader.Read(fixSource); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got := clipboardText(t, r); got != "" {
		t.Fatalf("clipboard = %q, want empty", got)
	}
}
