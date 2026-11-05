package files

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeFiles answers the index from fixed lists and records the search it was asked.
type fakeFiles struct {
	api.Files
	recent   []api.IndexedFile
	matches  []api.IndexedFile
	lastText string
	lastKind string
}

func (f *fakeFiles) Recent(options api.IndexOptions) []api.IndexedFile {
	f.lastKind = options.Kind
	return f.recent
}

func (f *fakeFiles) SearchIndex(query string, options api.IndexOptions) []api.IndexedFile {
	f.lastText, f.lastKind = query, options.Kind
	return f.matches
}

func (f *fakeFiles) Thumbnail(path string) (string, bool) { return "data:image/png;base64,AAAA", true }

type fakeDesktop struct {
	api.Desktop
	files *fakeFiles
}

func (d fakeDesktop) Files() api.Files { return d.files }

type fakeSystem struct {
	api.System
	quickLook bool
}

func (s fakeSystem) HasCapability(name string) bool { return s.quickLook && name == "quick-look" }

func (s fakeSystem) Labels() api.SystemLabels {
	return api.SystemLabels{PreviewFile: "Quick Look", RevealInFileManager: "Show in Finder"}
}

type fakeAccount struct{ signed bool }

func (a fakeAccount) SignedIn() bool { return a.signed }

type fakeContext struct {
	api.Context
	files  *fakeFiles
	system fakeSystem
	signed bool
}

func (c fakeContext) Desktop() api.Desktop { return fakeDesktop{files: c.files} }
func (c fakeContext) System() api.System   { return c.system }
func (c fakeContext) Account() api.Account { return fakeAccount{signed: c.signed} }

func indexed(path, name, kind string) api.IndexedFile {
	return api.IndexedFile{ID: path, Path: path, Name: name, DisplayPath: "~/" + name, Kind: kind}
}

func TestRootShowsTheSixNewestFilesWithTheirOwnMenu(t *testing.T) {
	files := &fakeFiles{recent: []api.IndexedFile{indexed("/h/a.pdf", "a.pdf", "file"), indexed("/h/b.png", "b.png", "image")}}
	ctx := fakeContext{files: files, system: fakeSystem{quickLook: true}}
	items, err := Extension{}.RootItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Score != 6 || items[1].Score != 5 {
		t.Fatalf("root = %+v, want two rows scored 6 and 5", items)
	}
	if items[0].Primary == nil || items[0].Primary.Kind != api.ActionOpenPath || items[0].Primary.Path != "/h/a.pdf" {
		t.Fatalf("primary = %+v, want Open of the file", items[0].Primary)
	}
	titles := menuTitles(items[0].Actions)
	if len(titles) != 3 || titles[0] != "Preview Contents" || titles[1] != "Quick Look" || titles[2] != "Show in Finder" {
		t.Fatalf("menu = %v, want Preview Contents, Quick Look, Show in Finder", titles)
	}
	if items[1].Actions[0].Title != "Preview" {
		t.Fatalf("picture preview title = %q, want Preview", items[1].Actions[0].Title)
	}
	if items[0].File == nil || items[0].File.Path != "/h/a.pdf" {
		t.Fatal("the row does not drag out its file")
	}
}

func TestRootTypedQueryNeedsTwoCharactersAndScoresFromSeventyFive(t *testing.T) {
	files := &fakeFiles{matches: []api.IndexedFile{indexed("/h/x.md", "x.md", "file"), indexed("/h/y.md", "y.md", "file")}}
	ctx := fakeContext{files: files}
	if items, _ := (Extension{}).SearchItems(ctx, "x"); items != nil {
		t.Fatalf("one character searched the index: %+v", items)
	}
	items, _ := Extension{}.SearchItems(ctx, "xy")
	if files.lastText != "xy" || len(items) != 2 || items[0].Score != 75 || items[1].Score != 74 {
		t.Fatalf("search = %+v for %q, want two rows from 75", items, files.lastText)
	}
}

func TestMeaningCommandIsOnlyListedWhileSignedIn(t *testing.T) {
	signedOut, _ := Extension{}.Commands(fakeContext{})
	signedIn, _ := Extension{}.Commands(fakeContext{signed: true})
	if len(signedOut) != 1 || signedOut[0].ID != searchCommandID {
		t.Fatalf("signed out commands = %+v, want only Search Files", signedOut)
	}
	if len(signedIn) != 2 || signedIn[1].ID != meaningCommandID {
		t.Fatalf("signed in commands = %+v, want Find Files by Meaning too", signedIn)
	}
}

func TestSearchChangedReplacesRowsAndTheKindKeepsTheQuery(t *testing.T) {
	files := &fakeFiles{matches: []api.IndexedFile{indexed("/h/x.png", "x.png", "image")}}
	ctx := fakeContext{files: files}
	ext := Extension{state: &viewState{}}
	result := ext.searchChanged(ctx, "x")
	if result.Patch == nil || result.Patch.Mode != api.PatchReplace || len(result.Patch.SearchResultIds) != 1 {
		t.Fatalf("search patch = %+v, want a replace with the match ids", result.Patch)
	}
	ext.kindChanged(ctx, "image")
	if files.lastText != "x" || files.lastKind != "image" {
		t.Fatalf("kind change searched %q as %q, want the last query as an image search", files.lastText, files.lastKind)
	}
}

func TestMeaningRowsAskOnlyForTheQueryTheUserAsked(t *testing.T) {
	files := &fakeFiles{matches: []api.IndexedFile{indexed("/h/tax.pdf", "tax.pdf", "file")}}
	ctx := fakeContext{files: files, signed: true}
	ext := Extension{state: &viewState{}}
	rows := ext.meaningRows(ctx, "tax")
	if len(rows) != 2 || rows[1].ID != meaningAskID || rows[1].Primary == nil || rows[1].Primary.Ref.Args[argQuery] != "tax" {
		t.Fatalf("rows before the ask = %+v, want the name match and the ask row for tax", rows)
	}
	ext.state.recordDecision("tax", []api.Item{{ID: "decided"}})
	rows = ext.meaningRows(ctx, "tax")
	if len(rows) != 2 || rows[1].ID != "decided" {
		t.Fatalf("rows after the ask = %+v, want the decision rows and no ask row", rows)
	}
	if other := ext.meaningRows(ctx, "receipt"); len(other) != 2 || other[1].ID != meaningAskID {
		t.Fatalf("a new query = %+v, want the ask row again", other)
	}
}

func menuTitles(actions []api.Action) []string {
	var titles []string
	for _, action := range actions {
		titles = append(titles, action.Title)
	}
	return titles
}
