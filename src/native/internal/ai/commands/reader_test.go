package commands

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// readerRig is a reader over fakes, with the marker fixed so a test can see what the clipboard held.
type readerRig struct {
	set    *fake.Set
	reader *Reader
}

func newReaderRig(t *testing.T, focus platform.Focus) readerRig {
	t.Helper()
	set := fake.NewSet()
	if focus == nil {
		focus = set.Focus
	}
	reader := NewReader(set.Clock, set.Clipboard, set.SelectedText, focus)
	reader.marker = func() string { return "__MARKER__" }
	return readerRig{set: set, reader: reader}
}

// TestReadCopiesTheSelectionAndRestoresTheClipboard reads a selection with text already on the clipboard; the copy
// must not leave the selection behind.
func TestReadCopiesTheSelectionAndRestoresTheClipboard(t *testing.T) {
	r := newReaderRig(t, nil)
	_ = r.set.Clipboard.WriteText("keep me")
	r.set.SelectedText.Select("this are selected text")

	got, err := r.reader.Read(fixSource)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "this are selected text" {
		t.Fatalf("Read = %q, want the selection", got)
	}
	if text, _ := r.set.Clipboard.ReadText(); text != "keep me" {
		t.Fatalf("clipboard after Read = %q, want the previous text back", text)
	}
	if reqs := r.set.SelectedText.Requests(); len(reqs) != 1 {
		t.Fatalf("copy gestures = %v, want one", reqs)
	}
	if focused := r.set.Focus.Restored(); len(focused) != 1 || focused[0] != fixSource {
		t.Fatalf("focus restored to %v, want the source window before the copy", focused)
	}
}

// TestReadWithNothingSelectedReturnsEmptyAfterTheCopyPolls covers a copy that never lands: the marker stays on the
// clipboard for every poll, and the clipboard comes back as it was.
func TestReadWithNothingSelectedReturnsEmptyAfterTheCopyPolls(t *testing.T) {
	r := newReaderRig(t, nil)
	_ = r.set.Clipboard.WriteText("before")
	done := make(chan struct{})
	var got string
	var err error
	go func() {
		got, err = r.reader.Read(fixSource)
		close(done)
	}()
	for i := 1; i < pollAttempts; i++ {
		r.set.Clock.AwaitTimerDueIn(pollInterval)
		r.set.Clock.Advance(pollInterval)
	}
	<-done
	if err != nil || got != "" {
		t.Fatalf("Read = %q, %v; want empty and no error", got, err)
	}
	if text, _ := r.set.Clipboard.ReadText(); text != "before" {
		t.Fatalf("clipboard = %q, want the previous text back", text)
	}
}

// TestReadRefusesFormattedClipboards: a clipboard with an image or files cannot be put back exactly, so nothing is
// copied over it.
func TestReadRefusesFormattedClipboards(t *testing.T) {
	r := newReaderRig(t, nil)
	r.set.Clipboard.SetFilePaths("/tmp/a.txt")
	r.set.SelectedText.Select("text")

	_, err := r.reader.Read(fixSource)
	if !errors.Is(err, errFormattedClipboard) {
		t.Fatalf("Read error = %v, want the formatted-clipboard refusal", err)
	}
	if reqs := r.set.SelectedText.Requests(); len(reqs) != 0 {
		t.Fatalf("copy gestures = %v, want none before the refusal", reqs)
	}
}

// TestReadReportsAGoneSourceWindow: the source window has closed, so there is nothing to copy from.
func TestReadReportsAGoneSourceWindow(t *testing.T) {
	r := newReaderRig(t, goneFocus{})
	r.set.SelectedText.Select("text")
	if _, err := r.reader.Read(fixSource); !errors.Is(err, errSourceGone) {
		t.Fatalf("Read error = %v, want errSourceGone", err)
	}
}

// TestReadKeepsTheClipboardWhenTheGestureFails: an OS that refuses the copy gesture still gets its clipboard back.
func TestReadKeepsTheClipboardWhenTheGestureFails(t *testing.T) {
	r := newReaderRig(t, nil)
	_ = r.set.Clipboard.WriteText("before")
	r.set.SelectedText.FailNext(fake.OpCopySelection, platform.ErrDenied)

	if _, err := r.reader.Read(fixSource); !errors.Is(err, platform.ErrDenied) {
		t.Fatalf("Read error = %v, want ErrDenied", err)
	}
	if text, _ := r.set.Clipboard.ReadText(); text != "before" {
		t.Fatalf("clipboard = %q, want the previous text back", text)
	}
}

// goneFocus is a focus port whose source window has closed.
type goneFocus struct{}

func (goneFocus) Active() (platform.FocusTarget, error) { return platform.FocusTarget{}, platform.ErrNotFound }
func (goneFocus) Restore(platform.FocusTarget) error     { return platform.ErrNotFound }
