package clipboard

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// linkEntry is a copy of a single web address, the kind a link preview is kept for.
func linkEntry(url string) Entry {
	return Entry{ID: TextID(url), Kind: KindText, Text: url}
}

// TestPreviewIsSavedAndKeptWhenTheAddressIsCopiedAgain: a preview saved with the history survives a second copy
// of the same address, which replaces the entry.
func TestPreviewIsSavedAndKeptWhenTheAddressIsCopiedAgain(t *testing.T) {
	h := newHarness(t)
	url := "https://example.com/post"
	h.mustRemember(linkEntry(url))
	if err := h.history.SetPreview(url, LinkPreview{URL: url, Title: "Post", ImageRef: ImageName(strings.Repeat("a", 40))}); err != nil {
		t.Fatalf("SetPreview: %v", err)
	}
	h.mustRemember(linkEntry(url))
	got, ok := h.history.PreviewFor(url)
	if !ok || got.Title != "Post" {
		t.Fatalf("preview after re-copy = %+v, %v", got, ok)
	}
}

// TestSavedPreviewPictureCountsAsAReference: a preview picture is kept by garbage collection like an image entry's.
func TestSavedPreviewPictureCountsAsAReference(t *testing.T) {
	h := newHarness(t)
	url := "https://example.com/picture"
	name := ImageName(strings.Repeat("b", 40))
	h.mustRemember(linkEntry(url))
	if err := h.history.SetPreview(url, LinkPreview{URL: url, Title: "Picture", ImageRef: name}); err != nil {
		t.Fatalf("SetPreview: %v", err)
	}
	if !h.history.HasImage(name) {
		t.Fatal("the preview picture is not referenced")
	}
	names, err := h.history.ImageNames()
	if err != nil || !names[name] {
		t.Fatalf("ImageNames = %v, %v; want the preview picture", names, err)
	}
	if _, err := h.history.Delete(linkEntry(url).ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if h.history.HasImage(name) {
		t.Fatal("the picture stays referenced after its entry is gone")
	}
}

// TestSetPreviewLeavesOtherEntriesAlone: only the entries of that address get the preview.
func TestSetPreviewLeavesOtherEntriesAlone(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(linkEntry("https://example.com/a"))
	h.mustRemember(Entry{ID: TextID("plain words"), Kind: KindText, Text: "plain words"})
	if err := h.history.SetPreview("https://example.com/a", LinkPreview{URL: "https://example.com/a", Title: "A"}); err != nil {
		t.Fatalf("SetPreview: %v", err)
	}
	for _, e := range h.history.Find(Query{}) {
		if e.Text == "plain words" && e.LinkPreview != nil {
			t.Fatal("a plain text entry received a link preview")
		}
	}
	if _, ok := h.history.PreviewFor("https://example.com/other"); ok {
		t.Fatal("a preview for another address was found")
	}
}

// TestSetRetentionAppliesAtOnce: a smaller limit removes the oldest unpinned entries now.
func TestSetRetentionAppliesAtOnce(t *testing.T) {
	h := newHarness(t)
	for _, text := range []string{"one", "two", "three", "four"} {
		h.mustRemember(Entry{ID: TextID(text), Kind: KindText, Text: text})
	}
	removed, err := h.history.SetRetention(Retention{MaxEntries: 2, MaxAge: -1})
	if err != nil || removed != 2 {
		t.Fatalf("SetRetention = %d, %v; want 2 removed", removed, err)
	}
	if got := ids(h.history.Find(Query{})); len(got) != 2 {
		t.Fatalf("entries = %v", got)
	}
}

// TestStorePersistenceRoundTripsThroughTheStoreDocument: the history is saved in clipboard.json by the store and
// read back by a new history over the same files.
func TestStorePersistenceRoundTripsThroughTheStoreDocument(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	dir := "/config/nvm-native"
	s, err := store.Open(files, clock, dir, store.WithDebounce(0))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	doc := NewSnapshotDocument(s)
	h, err := Open(NewStorePersistence(doc), clock, Retention{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := h.Remember(Entry{ID: TextID("kept"), Kind: KindText, Text: "kept"}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, _ := files.ReadFile(dir + "/" + SnapshotDocument); !strings.Contains(string(got), "kept") {
		t.Fatalf("clipboard.json does not hold the entry: %s", got)
	}

	s2, err := store.Open(files, clock, dir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	reopened, err := Open(NewStorePersistence(NewSnapshotDocument(s2)), clock, Retention{})
	if err != nil {
		t.Fatalf("reopen history: %v", err)
	}
	if got := reopened.Find(Query{}); len(got) != 1 || got[0].Text != "kept" {
		t.Fatalf("reloaded = %+v", got)
	}
}

// TestStorePersistenceOpensANewerDocumentReadOnly: a clipboard.json written by a newer build is never overwritten.
func TestStorePersistenceOpensANewerDocumentReadOnly(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	dir := "/config/nvm-native"
	newer := `{"schemaVersion":2,"data":{"version":2,"entries":[{"id":"text:x","kind":"text","text":"x"}]}}`
	files.Seed(dir+"/"+SnapshotDocument, []byte(newer))
	s, err := store.Open(files, clock, dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	h, err := Open(NewStorePersistence(NewSnapshotDocument(s)), clock, Retention{})
	if !errors.Is(err, ErrNewerSnapshot) || h == nil {
		t.Fatalf("Open = %v, %v; want the read-only history", h, err)
	}
	if err := h.Remember(Entry{ID: TextID("y"), Kind: KindText, Text: "y"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Remember = %v, want ErrReadOnly", err)
	}
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, _ := files.ReadFile(dir + "/" + SnapshotDocument); string(got) != newer {
		t.Fatalf("the newer document was overwritten: %s", got)
	}
}
