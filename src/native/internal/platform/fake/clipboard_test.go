package fake

import (
	"errors"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestClipboardBurstOfWritesCoalescesIntoOneNotification(t *testing.T) {
	clip := NewClipboard()
	for _, text := range []string{"a", "b", "c"} {
		if err := clip.WriteText(text); err != nil {
			t.Fatalf("WriteText: %v", err)
		}
	}
	<-clip.Changed()
	select {
	case <-clip.Changed():
		t.Fatal("a second notification arrived for a burst that was not read in between")
	default:
	}
	if got, _ := clip.ReadText(); got != "c" {
		t.Fatalf("ReadText = %q, want the last write", got)
	}
}

func TestClipboardWritesReplaceEachOther(t *testing.T) {
	clip := NewClipboard()
	if err := clip.WriteImagePNG([]byte("png-bytes")); err != nil {
		t.Fatalf("WriteImagePNG: %v", err)
	}
	if err := clip.WriteText("text"); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if _, err := clip.ReadImagePNG(); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("ReadImagePNG after text write: %v, want ErrNotFound", err)
	}
	formats, _ := clip.Formats()
	if !reflect.DeepEqual(formats, []platform.ClipboardFormat{platform.FormatText}) {
		t.Fatalf("Formats = %v, want only text", formats)
	}
}

func TestConcealedCopyIsReportedAsConcealed(t *testing.T) {
	clip := NewClipboard()
	clip.WriteConcealedText("hunter2")
	formats, err := clip.Formats()
	if err != nil {
		t.Fatalf("Formats: %v", err)
	}
	if !reflect.DeepEqual(formats, []platform.ClipboardFormat{platform.FormatText, platform.FormatConcealed}) {
		t.Fatalf("Formats = %v, want text and concealed", formats)
	}
	if err := clip.WriteText("plain"); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	formats, _ = clip.Formats()
	if !reflect.DeepEqual(formats, []platform.ClipboardFormat{platform.FormatText}) {
		t.Fatalf("Formats after a plain copy = %v, want text only", formats)
	}
}

func TestClipboardImageReturnsCopyAndClearEmptiesEverything(t *testing.T) {
	clip := NewClipboard()
	source := []byte("png")
	if err := clip.WriteImagePNG(source); err != nil {
		t.Fatalf("WriteImagePNG: %v", err)
	}
	source[0] = 'X'
	got, err := clip.ReadImagePNG()
	if err != nil || string(got) != "png" {
		t.Fatalf("ReadImagePNG = %q, %v", got, err)
	}
	<-clip.Changed()
	if err := clip.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	<-clip.Changed()
	formats, _ := clip.Formats()
	if len(formats) != 0 {
		t.Fatalf("Formats after Clear = %v", formats)
	}
}

func TestClipboardFaultInjectionFailsWithoutChangingContent(t *testing.T) {
	clip := NewClipboard()
	clip.WriteConcealedText("kept")
	boom := errors.New("pasteboard busy")
	clip.FailNext(OpClipboardWriteText, boom)
	if err := clip.WriteText("lost"); !errors.Is(err, boom) {
		t.Fatalf("WriteText = %v, want injected error", err)
	}
	if got, _ := clip.ReadText(); got != "kept" {
		t.Fatalf("ReadText after failed write = %q", got)
	}
}
