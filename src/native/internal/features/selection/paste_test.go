package selection

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// TestPasteWritesTheClipboardAndRestoresItAfterTheDelay: the paste lands, the pasted text stays until the delay passes,
// and then the clipboard the user had comes back.
func TestPasteWritesTheClipboardAndRestoresItAfterTheDelay(t *testing.T) {
	rig := newEngineRig(t)
	if err := rig.set.Clipboard.WriteText("keep me"); err != nil {
		t.Fatal(err)
	}
	options := PasteOptions{RestoreClipboard: true, RestoreDelay: 100 * time.Millisecond}
	if err := rig.engine.Paste(Content{Kind: KindText, Text: "hi"}, options); err != nil {
		t.Fatalf("Paste = %v; want the paste to land", err)
	}
	if got := rig.set.Paster.Requests(); len(got) != 1 {
		t.Fatalf("paste requests = %v, want one", got)
	}
	rig.set.Clock.AwaitTimerDueIn(100 * time.Millisecond)
	rig.set.Clock.Advance(100 * time.Millisecond)
	waitClipboard(t, rig.set.Clipboard, "keep me")
}

// TestPasteRestoresTheClipboardAtOnceWhenThePasteFails: a refused paste must not leave the replacement on the clipboard.
func TestPasteRestoresTheClipboardAtOnceWhenThePasteFails(t *testing.T) {
	rig := newEngineRig(t)
	if err := rig.set.Clipboard.WriteText("keep me"); err != nil {
		t.Fatal(err)
	}
	rig.set.Paster.FailNext("PasteIntoFrontmost", platform.ErrConflict)
	err := rig.engine.Paste(Content{Kind: KindText, Text: "hi"}, PasteOptions{RestoreClipboard: true, RestoreDelay: time.Second})
	if !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("Paste = %v; want ErrConflict", err)
	}
	if got, _ := rig.set.Clipboard.ReadText(); got != "keep me" {
		t.Fatalf("clipboard = %q after a refused paste; want it back", got)
	}
}

// TestConcealedImagePasteIsRefused: an image cannot be marked concealed, so a paste that must keep it out of history
// does not put it on the clipboard.
func TestConcealedImagePasteIsRefused(t *testing.T) {
	rig := newEngineRig(t)
	if err := rig.set.Clipboard.WriteText("before"); err != nil {
		t.Fatal(err)
	}
	err := rig.engine.Paste(Content{Kind: KindImage, PNG: []byte("\x89PNG")}, PasteOptions{Concealed: true})
	if !errors.Is(err, ErrConcealedContent) {
		t.Fatalf("Paste = %v; want ErrConcealedContent", err)
	}
	if got, _ := rig.set.Clipboard.ReadText(); got != "before" {
		t.Fatalf("clipboard = %q; a refused image must not reach it", got)
	}
}

// TestTypeTextTypesIntoTheSourceWithoutTheClipboard: typing sends key presses and leaves the clipboard alone.
func TestTypeTextTypesIntoTheSourceWithoutTheClipboard(t *testing.T) {
	rig := newEngineRig(t)
	if err := rig.set.Clipboard.WriteText("untouched"); err != nil {
		t.Fatal(err)
	}
	if err := rig.engine.TypeText("typed", 5*time.Millisecond); err != nil {
		t.Fatalf("TypeText = %v; want the keys sent", err)
	}
	if got := rig.set.Paster.Typed(); len(got) != 1 || got[0] != "typed" {
		t.Fatalf("typed = %v; want typed", got)
	}
	if got, _ := rig.set.Clipboard.ReadText(); got != "untouched" {
		t.Fatalf("clipboard = %q; typing must not touch it", got)
	}
}

// TestTypeTextIsUnsupportedWithoutATypist: a paster that cannot type refuses, rather than pasting in its place.
func TestTypeTextIsUnsupportedWithoutATypist(t *testing.T) {
	rig := newEngineRig(t)
	rig.engine.deps.Paster = nil
	if err := rig.engine.TypeText("typed", 0); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("TypeText without a paster = %v; want ErrUnsupported", err)
	}
}
