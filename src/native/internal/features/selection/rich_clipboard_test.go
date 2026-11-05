package selection

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestRichClipboardSurvivesCaptureReplaceAndRefusedGestures(t *testing.T) {
	for _, kind := range []string{"html", "image", "files", "concealed"} {
		t.Run(kind, func(t *testing.T) {
			r := newEngineRig(t)
			switch kind {
			case "html":
				_ = r.set.Clipboard.WriteRich("original", "<b>original</b>")
			case "image":
				_ = r.set.Clipboard.WriteImagePNG([]byte{1, 2, 3})
			case "files":
				r.set.Clipboard.SetFilePaths("/one", "/two")
			case "concealed":
				_ = r.set.Clipboard.WriteConcealedText("private")
			}
			before, _ := r.set.Clipboard.Snapshot()
			r.set.SelectedText.Select("selected")
			text, found, err := r.engine.Text()
			if err != nil || !found || text != "selected" {
				t.Fatalf("capture: %q %v %v", text, found, err)
			}
			assertSnapshot(t, r.set.Clipboard, before)
			r.set.SelectedText.FailNext(fake.OpCopySelection, platform.ErrDenied)
			if _, _, err := r.engine.Text(); !errors.Is(err, platform.ErrDenied) {
				t.Fatalf("copy failure = %v", err)
			}
			assertSnapshot(t, r.set.Clipboard, before)
			r.set.Paster.FailNext(fake.OpPasteIntoFrontmost, platform.ErrDenied)
			if _, err := r.engine.Replace("fixed"); !errors.Is(err, platform.ErrDenied) {
				t.Fatalf("replace failure = %v", err)
			}
			assertSnapshot(t, r.set.Clipboard, before)
			if _, err := r.engine.Replace("fixed"); err != nil {
				t.Fatal(err)
			}
			r.set.Clock.AwaitTimerDueIn(500 * time.Millisecond)
			r.set.Clock.Advance(500 * time.Millisecond)
			awaitSnapshot(t, r.set.Clipboard, before)
		})
	}
}

func TestDelayedRestoreKeepsANewerUserCopyEvenWithIdenticalText(t *testing.T) {
	r := newEngineRig(t)
	_ = r.set.Clipboard.WriteRich("original", "<b>original</b>")
	observed := &observedRestore{Clipboard: r.set.Clipboard, done: make(chan struct{})}
	r.engine.reader.clip = observed
	r.engine.deps.Clipboard = observed
	if err := r.engine.Paste(Content{Kind: KindText, Text: "transcript"}, PasteOptions{RestoreClipboard: true}); err != nil {
		t.Fatal(err)
	}
	r.set.Clock.AwaitTimerDueIn(500 * time.Millisecond)
	_ = r.set.Clipboard.WriteConcealedText("transcript")
	newer, _ := r.set.Clipboard.Snapshot()
	r.set.Clock.Advance(500 * time.Millisecond)
	select {
	case <-observed.done:
	case <-time.After(2 * time.Second):
		t.Fatal("delayed restoration did not finish")
	}
	assertSnapshot(t, r.set.Clipboard, newer)
}

func TestExplicitPasteTargetDoesNotRestoreAStaleSummonedWindow(t *testing.T) {
	r := newEngineRig(t)
	if err := r.engine.Paste(Content{Kind: KindText, Text: "transcript"}, PasteOptions{ExpectedAppID: "source-app"}); err != nil {
		t.Fatal(err)
	}
	if len(r.set.Focus.Restored()) != 0 || !reflect.DeepEqual(r.set.Paster.Requests(), []string{"source-app"}) {
		t.Fatal("an explicit target restored the window from an unrelated palette summon")
	}
}

func assertSnapshot(t *testing.T, clip *fake.Clipboard, want platform.ClipboardSnapshot) {
	t.Helper()
	got, err := clip.Snapshot()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("clipboard changed: got=%+v want=%+v err=%v", got, want, err)
	}
}

func awaitSnapshot(t *testing.T, clip *fake.Clipboard, want platform.ClipboardSnapshot) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, _ := clip.Snapshot()
		if reflect.DeepEqual(got, want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("clipboard did not restore: got=%+v want=%+v", got, want)
		}
		time.Sleep(time.Millisecond)
	}
}

type observedRestore struct {
	*fake.Clipboard
	done chan struct{}
}

func (c *observedRestore) RestoreIfOwned(snapshot platform.ClipboardSnapshot, owner platform.ClipboardOwnership) error {
	err := c.Clipboard.RestoreIfOwned(snapshot, owner)
	close(c.done)
	return err
}

func TestDelayedCopyKeepsTheRichSnapshotUntilCaptureCompletes(t *testing.T) {
	r := newReaderRig(t, nil)
	_ = r.set.Clipboard.WriteRich("original", "<b>original</b>")
	before, _ := r.set.Clipboard.Snapshot()
	r.reader.selection = pendingCopy{}
	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() { text, err := r.reader.Read(fixSource); done <- result{text, err} }()
	r.set.Clock.AwaitTimerDueIn(pollInterval)
	_ = r.set.Clipboard.WriteText("late selection")
	r.set.Clock.Advance(pollInterval)
	select {
	case got := <-done:
		if got.err != nil || got.text != "late selection" {
			t.Fatalf("delayed capture = %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("delayed capture never finished")
	}
	assertSnapshot(t, r.set.Clipboard, before)
}

type pendingCopy struct{}

func (pendingCopy) CopySelection(string) error { return nil }

func TestOverlappingPastesRestoreTheOriginalRichClipboard(t *testing.T) {
	r := newEngineRig(t)
	_ = r.set.Clipboard.WriteRich("original", "<b>original</b>")
	before, _ := r.set.Clipboard.Snapshot()
	for _, text := range []string{"first", "second"} {
		if err := r.engine.Paste(Content{Kind: KindText, Text: text}, PasteOptions{RestoreClipboard: true}); err != nil {
			t.Fatal(err)
		}
	}
	r.set.Clock.AwaitTimers(2)
	r.set.Clock.Advance(500 * time.Millisecond)
	awaitSnapshot(t, r.set.Clipboard, before)
}
