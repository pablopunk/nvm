package fake

import (
	"reflect"
	"testing"
)

func TestSnapshotRestoresEveryStoredRepresentationAndMarker(t *testing.T) {
	clip := NewClipboard()
	clip.text, clip.html, clip.rtf = "plain", "<b>plain</b>", `{\rtf1 plain}`
	clip.png, clip.files, clip.concealed = []byte{1, 2, 3}, []string{"/one", "/two"}, true
	before, err := clip.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := clip.WriteTemporaryText("temporary")
	if err != nil {
		t.Fatal(err)
	}
	if err := clip.RestoreIfOwned(before, owner); err != nil {
		t.Fatal(err)
	}
	after, err := clip.Snapshot()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("snapshot changed: before=%+v after=%+v err=%v", before, after, err)
	}
}

func TestSnapshotDoesNotOverwriteANewerCopyWithIdenticalText(t *testing.T) {
	clip := NewClipboard()
	_ = clip.WriteRich("original", "<b>original</b>")
	snapshot, _ := clip.Snapshot()
	owner, _ := clip.WriteTemporaryText("same text")
	_ = clip.WriteConcealedText("same text")
	if err := clip.RestoreIfOwned(snapshot, owner); err != nil {
		t.Fatal(err)
	}
	text, _ := clip.ReadText()
	html, _ := clip.ReadHTML()
	if text != "same text" || html != "" {
		t.Fatal("restoration overwrote a newer copy with the same text and formats")
	}
}
