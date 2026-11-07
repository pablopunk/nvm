package extview

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// searchFilesChrome is a list shaped as the Search Files view is: a footer status and hint, and a kind accessory that Tab
// steps through.
var searchFilesChrome = api.View{
	Kind: api.ViewList, Title: "Search Files", Hint: "Tab kind", Status: "All files · 3 files indexed",
	SearchAccessory: &api.SearchAccessory{
		Options: []api.SearchAccessoryOption{{Title: "All files", Value: ""}, {Title: "Images", Value: "image"}, {Title: "Videos", Value: "video"}},
		Value:   "", CycleOnTab: true, OnChange: &api.ActionRef{ID: "kind"},
	},
}

// TestTabChoosesTheNextKindRunsItsHandlerAndWraps checks Tab steps through the accessory's options in order, running the
// handler for each, and wraps after the last one.
func TestTabChoosesTheNextKindRunsItsHandlerAndWraps(t *testing.T) {
	log := &invokeLog{results: map[string]api.Result{"kind": {}}}
	page := NewSession(log.deps(fake.NewFS()), "nevermind.files", searchFilesChrome).listPage()
	p := palette.New(palette.Options{Root: &palette.View{Title: "Nevermind"}, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	p.Show()
	p.Push(page)
	tt := ui.NewTester(p.View, 720, 470)
	tt.Frame()
	for _, want := range []string{"image", "video", ""} {
		tt.Key(0, ui.KeyTab)
		tt.Frame()
		if page.Accessory.Value != want {
			t.Fatalf("accessory value = %q after Tab, want %q", page.Accessory.Value, want)
		}
	}
	if !slices.Equal(log.calls, []string{"nevermind.files:kind", "nevermind.files:kind", "nevermind.files:kind"}) {
		t.Fatalf("handler calls = %v, want one kind change per Tab", log.calls)
	}
	if !tt.HasText("All files · 3 files indexed") {
		t.Fatal("the footer does not show the view's status")
	}
}

// TestStatusPatchRetitlesTheFooterAndTheEmptyState checks a refresh's status and empty state replace the footer and the
// empty list, and that a patch without them leaves both alone.
func TestStatusPatchRetitlesTheFooterAndTheEmptyState(t *testing.T) {
	session := NewSession(Deps{}, "nevermind.files", searchFilesChrome)
	session.Apply(api.ViewPatch{Status: "Images · 9 files indexed", EmptyView: &api.EmptyState{Title: "No files found."}})
	if got := session.footerName(nil); got != "Images · 9 files indexed" {
		t.Fatalf("footer = %q after the patch", got)
	}
	if title, _ := session.emptyState(); title != "No files found." {
		t.Fatalf("empty title = %q after the patch", title)
	}
	session.Apply(api.ViewPatch{})
	if got := session.footerName(nil); got != "Images · 9 files indexed" {
		t.Fatalf("footer = %q after an empty patch, want it kept", got)
	}
}

func TestFilePreviewReadsLeadingTextAndNotesWhatItLeaves(t *testing.T) {
	files := fake.NewFS()
	if err := files.MkdirAll("/h", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile("/h/notes.txt", []byte("buy milk"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile("/h/blob.bin", append([]byte{0, 1, 2}, bytes.Repeat([]byte{7}, 100)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile("/h/long.txt", bytes.Repeat([]byte("a"), 70<<10), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewSession(Deps{Files: files}, "nevermind.files", api.View{})
	if got := s.leadingText(api.FileRef{Path: "/h/notes.txt", Size: 8}); got.text != "buy milk" || got.note != "" {
		t.Fatalf("text preview = %+v, want the text and no note", got)
	}
	if got := s.leadingText(api.FileRef{Path: "/h/blob.bin", Size: 103}); got.text != "" || !strings.HasPrefix(got.note, "Binary file (") {
		t.Fatalf("binary preview = %+v, want the binary note", got)
	}
	if got := s.leadingText(api.FileRef{Path: "/h/long.txt", Size: 70 << 10}); !strings.HasSuffix(got.note, "Showing the first 64 KB of 70 KB.") {
		t.Fatalf("long preview note = %q, want the truncation note", got.note)
	}
}

func TestFileMetadataNamesTheKindSizeModifiedTimeAndPath(t *testing.T) {
	lines := fileMetadataLines(api.FileRef{Kind: "image", Size: 2 << 20, DisplayPath: "~/a.png"})
	if !slices.Equal(lines, []string{"Kind: Image", "Size: 2.0 MB", "Path: ~/a.png"}) {
		t.Fatalf("metadata = %v", lines)
	}
}
