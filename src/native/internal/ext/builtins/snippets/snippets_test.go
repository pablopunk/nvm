package snippets

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

var testNow = time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC)

// fakeStorage is an in-memory extension storage holding JSON values by key.
type fakeStorage struct {
	api.Storage
	values map[string][]byte
	failed bool
}

func (s *fakeStorage) Get(key string, into any) (bool, error) {
	raw, ok := s.values[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, into)
}

func (s *fakeStorage) Set(key string, value any) error {
	if s.failed {
		return errors.New("disk full")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.values[key] = raw
	return nil
}

// fakeClipboard holds the text the system clipboard has.
type fakeClipboard struct {
	api.SystemClipboard
	text    string
	written []string
}

func (c *fakeClipboard) ReadText() (string, error) { return c.text, nil }

func (c *fakeClipboard) WriteText(text string, _ bool) error {
	c.written = append(c.written, text)
	c.text = text
	return nil
}

type fakeDesktop struct {
	api.Desktop
	clipboard *fakeClipboard
}

func (d fakeDesktop) Clipboard() api.SystemClipboard { return d.clipboard }

type fakeSystem struct {
	api.System
	canPaste bool
}

func (s fakeSystem) HasCapability(name string) bool { return name == "clipboard.paste" && s.canPaste }

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

type fakeNavigation struct{ api.Navigation }

func (fakeNavigation) Push(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavPush}
}

func (fakeNavigation) Pop() api.Result { return api.Result{Navigation: api.NavPop} }

type fakeViews struct{ api.Views }

func (fakeViews) Refresh() api.Action { return api.Run("refresh", "Refresh", nil) }

// fakeContext is the context a test runs the built-in with: storage, clock, clipboard and the paste capability.
type fakeContext struct {
	api.Context
	storage   *fakeStorage
	clipboard *fakeClipboard
	canPaste  bool
}

func newContext() fakeContext {
	return fakeContext{storage: &fakeStorage{values: map[string][]byte{}}, clipboard: &fakeClipboard{}, canPaste: true}
}

func (c fakeContext) Storage() api.Storage       { return c.storage }
func (c fakeContext) Clock() api.Clock           { return fakeClock{now: testNow} }
func (c fakeContext) Desktop() api.Desktop       { return fakeDesktop{clipboard: c.clipboard} }
func (c fakeContext) System() api.System         { return fakeSystem{canPaste: c.canPaste} }
func (c fakeContext) Navigation() api.Navigation { return fakeNavigation{} }
func (c fakeContext) Views() api.Views           { return fakeViews{} }

// run runs one of the built-in's actions with the given args and form.
func run(t *testing.T, ext Extension, ctx api.Context, ref string, args map[string]string, form api.FormValues) (api.Result, error) {
	t.Helper()
	return ext.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: ref, Args: args}, Form: form})
}

// saveForm is the form values a save submits.
func saveForm(name, keyword, text string) api.FormValues {
	return api.FormValues{fieldName: api.TextValue(name), fieldKeyword: api.TextValue(keyword), fieldContent: api.TextValue(text)}
}

// save runs a create (id empty) or an update of id with the given fields.
func save(t *testing.T, ext Extension, ctx api.Context, id, name, keyword, text string) error {
	t.Helper()
	_, err := run(t, ext, ctx, refSave, map[string]string{argID: id}, saveForm(name, keyword, text))
	return err
}

// stored returns the snippets the storage holds under the snippets key.
func stored(t *testing.T, ctx fakeContext) []Snippet {
	t.Helper()
	var out []Snippet
	if _, err := ctx.storage.Get(storageKey, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCommandsAreListedAsBefore(t *testing.T) {
	cmds, err := New(nil).Commands(newContext())
	if err != nil || len(cmds) != 2 {
		t.Fatalf("commands = %d, %v", len(cmds), err)
	}
	list, create := cmds[0], cmds[1]
	if list.Title != "Snippets" || len(list.Placement) != 0 || !list.Customizable || list.Appearance.Foreground != api.ColorTeal {
		t.Fatalf("Snippets command = %+v", list)
	}
	if create.Title != "Create Snippet" || len(create.Placement) != 1 || create.Placement[0] != api.PlacementSearch {
		t.Fatalf("Create Snippet command = %+v", create)
	}
}

func TestKeywordFindsTheSnippetExactBeforeItsPrefixes(t *testing.T) {
	ctx := newContext()
	ext := New([]Snippet{
		{ID: "snip-a", Name: "Signature", Keyword: ";sig", Text: "Pablo"},
		{ID: "snip-b", Name: "Signature long", Keyword: ";sigfull", Text: "Pablo Varela"},
		{ID: "snip-c", Name: "Name only", Text: "text"},
	})
	rows, err := ext.SearchItems(ctx, ";sig")
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows = %d, %v", len(rows), err)
	}
	if rows[0].Title != "Signature" || rows[0].Score != scoreSnippetExact || rows[1].Score != scoreSnippetPrefix {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Section != "Snippets" || rows[0].LearnKey != "snippet:snip-a" {
		t.Fatalf("row presentation = %q %q", rows[0].Section, rows[0].LearnKey)
	}
}

func TestBlankQueryAndSnippetsWithoutKeywordFindNothingAtTheRoot(t *testing.T) {
	ext := New([]Snippet{{ID: "snip-a", Name: "Signature", Text: "Pablo"}})
	if rows, _ := ext.SearchItems(newContext(), ""); len(rows) != 0 {
		t.Fatalf("blank query rows = %d", len(rows))
	}
	if rows, _ := ext.SearchItems(newContext(), "sig"); len(rows) != 0 {
		t.Fatalf("keyword-less snippet found by keyword: %d rows", len(rows))
	}
}

func TestEnterPastesWhereThePlatformCanPasteAndCopiesWhereItCannot(t *testing.T) {
	legacy := []Snippet{{ID: "snip-a", Name: "Signature", Keyword: ";sig", Text: "Pablo"}}
	ctx := newContext()
	rows, _ := New(legacy).SearchItems(ctx, ";sig")
	if rows[0].Primary == nil || rows[0].Primary.Title != "Paste" {
		t.Fatalf("primary with paste = %+v", rows[0].Primary)
	}
	ctx.canPaste = false
	rows, _ = New(legacy).SearchItems(ctx, ";sig")
	if rows[0].Primary == nil || rows[0].Primary.Title != "Copy to Clipboard" {
		t.Fatalf("primary without paste = %+v", rows[0].Primary)
	}
}

func TestPasteFillsPlaceholdersFromTheClipboardAndHandsTheTextToThePasteFamily(t *testing.T) {
	ctx := newContext()
	ctx.clipboard.text = "https://example.com"
	ext := New([]Snippet{{ID: "snip-a", Name: "Link", Text: "See {clipboard} on {date}"}})
	result, err := run(t, ext, ctx, refPaste, map[string]string{argID: "snip-a"}, nil)
	if err != nil || result.Next == nil {
		t.Fatalf("paste = %+v, %v", result, err)
	}
	if result.Next.Kind != api.ActionPasteText || result.Next.Text != "See https://example.com on 2026-11-05" {
		t.Fatalf("paste action = %+v", result.Next)
	}
}

func TestCopyWritesTheFilledTextToTheClipboard(t *testing.T) {
	ctx := newContext()
	ctx.clipboard.text = "x"
	ext := New([]Snippet{{ID: "snip-a", Name: "Echo", Text: "{clipboard}{clipboard}"}})
	if _, err := run(t, ext, ctx, refCopy, map[string]string{argID: "snip-a"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(ctx.clipboard.written) != 1 || ctx.clipboard.written[0] != "xx" {
		t.Fatalf("written = %q", ctx.clipboard.written)
	}
}

func TestExpandFillsPlaceholdersInOnePass(t *testing.T) {
	got := Expand("{clipboard} {date} {unknown}", "{date}", testNow)
	if got != "{date} 2026-11-05 {unknown}" {
		t.Fatalf("expand = %q", got)
	}
}

func TestCreateValidatesEveryFieldAndNormalizesTheKeyword(t *testing.T) {
	cases := map[string]struct {
		name, keyword, text string
		want                string
	}{
		"no name":        {"  ", "", "text", "Name is required."},
		"no content":     {"Name", "", "  \n", "Content is required."},
		"keyword spaces": {"Name", "two words", "text", "A keyword is one word, with no spaces."},
		"name too long":  {strings.Repeat("n", maxNameRunes+1), "", "text", "That is too long for a snippet."},
	}
	for name, tc := range cases {
		err := save(t, New(nil), newContext(), "", tc.name, tc.keyword, tc.text)
		if err == nil || err.Error() != tc.want {
			t.Fatalf("%s: error = %v, want %q", name, err, tc.want)
		}
	}
	ctx := newContext()
	if err := save(t, New(nil), ctx, "", " Email ", " ;SIG ", "\nPablo\n"); err != nil {
		t.Fatal(err)
	}
	if got := stored(t, ctx); len(got) != 1 || got[0].Name != "Email" || got[0].Keyword != ";sig" || got[0].Text != "Pablo" {
		t.Fatalf("stored = %+v", got)
	}
}

func TestKeywordIsUniqueAndAnUpdateKeepsItsOwn(t *testing.T) {
	ctx := newContext()
	ext := New(nil)
	if err := save(t, ext, ctx, "", "First", ";a", "one"); err != nil {
		t.Fatal(err)
	}
	if err := save(t, ext, ctx, "", "Second", ";a", "two"); err == nil || !strings.Contains(err.Error(), "already used by “First”") {
		t.Fatalf("taken keyword error = %v", err)
	}
	first := stored(t, ctx)[0]
	if err := save(t, ext, ctx, first.ID, "First renamed", ";a", "one again"); err != nil {
		t.Fatalf("update keeping its keyword: %v", err)
	}
	if got := stored(t, ctx)[0]; got.Name != "First renamed" || got.Text != "one again" {
		t.Fatalf("updated = %+v", got)
	}
}

func TestLegacySnippetsAnswerUntilTheFirstSaveThenLiveInStorage(t *testing.T) {
	ctx := newContext()
	legacy := []Snippet{{ID: "snip-old", Name: "Old", Text: "kept"}, {ID: "bad", Name: "Malformed", Text: "x"}}
	ext := New(legacy)
	if got := ext.listSnippets(ctx); len(got) != 1 || got[0].ID != "snip-old" {
		t.Fatalf("legacy list = %+v", got)
	}
	if err := save(t, ext, ctx, "", "New", "", "text"); err != nil {
		t.Fatal(err)
	}
	if got := stored(t, ctx); len(got) != 3 {
		t.Fatalf("a save drops malformed entries: stored %d", len(got))
	}
}

func TestDeleteRemovesTheSnippetAndKeepsMalformedEntries(t *testing.T) {
	ctx := newContext()
	ext := New([]Snippet{{ID: "snip-a", Name: "A", Text: "a"}, {ID: "snip-b", Name: "B", Text: "b"}, {ID: "bad", Text: " "}})
	if _, err := run(t, ext, ctx, refDelete, map[string]string{argID: "snip-a"}, nil); err != nil {
		t.Fatal(err)
	}
	got := stored(t, ctx)
	if len(got) != 2 || got[0].ID != "snip-b" || got[1].ID != "bad" {
		t.Fatalf("after delete = %+v", got)
	}
	if _, err := run(t, ext, ctx, refDelete, map[string]string{argID: "snip-a"}, nil); !errors.Is(err, errNotFound) {
		t.Fatalf("delete of a missing snippet = %v", err)
	}
}

func TestStorageFailureIsReturnedNotHidden(t *testing.T) {
	ctx := newContext()
	ctx.storage.failed = true
	if err := save(t, New(nil), ctx, "", "Name", "", "text"); err == nil || err.Error() != "disk full" {
		t.Fatalf("save error = %v", err)
	}
}

func TestListViewHasTheCreateRowThenSnippetsByName(t *testing.T) {
	items := listItems([]Snippet{{ID: "snip-b", Name: "beta", Text: "b"}, {ID: "snip-a", Name: "Alpha", Text: "a"}}, true)
	if len(items) != 3 || items[0].Title != createTitle || items[1].Title != "Alpha" || items[2].Title != "beta" {
		t.Fatalf("list titles = %v", items)
	}
}

func TestUnknownActionIsRefused(t *testing.T) {
	if _, err := run(t, New(nil), newContext(), "nope", nil, nil); !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("unknown action error = %v", err)
	}
}
