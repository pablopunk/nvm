package clipboard

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

var testNow = time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC)

// fakeHistory is the clipboard history of a test: a list of entries, newest first, and what the calls removed.
type fakeHistory struct {
	api.ClipboardHistory
	entries  []api.ClipboardEntry
	queries  []api.ClipboardQuery
	removed  [][]string
	cleared  bool
	failList error
}

func (h *fakeHistory) List(query api.ClipboardQuery) ([]api.ClipboardEntry, error) {
	h.queries = append(h.queries, query)
	if h.failList != nil {
		return nil, h.failList
	}
	out := make([]api.ClipboardEntry, 0, len(h.entries))
	for _, e := range h.entries {
		if query.Text != "" && !contains(e.Text, query.Text) {
			continue
		}
		if len(query.Types) > 0 && !hasType(query.Types, e.Type) {
			continue
		}
		out = append(out, e)
		if query.Limit > 0 && len(out) == query.Limit {
			break
		}
	}
	return out, nil
}

func (h *fakeHistory) Get(id string) (api.ClipboardEntry, bool, error) {
	for _, e := range h.entries {
		if e.ID == id {
			return e, true, nil
		}
	}
	return api.ClipboardEntry{}, false, nil
}

func (h *fakeHistory) Remove(ids []string) (int, error) {
	h.removed = append(h.removed, append([]string(nil), ids...))
	return len(ids), nil
}

func (h *fakeHistory) Clear(types []string, olderThanMs int64) (int, error) {
	h.cleared = true
	return len(h.entries), nil
}

// fakeClipboard is the system clipboard of a test: what was written, in order.
type fakeClipboard struct {
	api.SystemClipboard
	written []api.ClipboardContent
}

func (c *fakeClipboard) Write(content api.ClipboardContent) error {
	c.written = append(c.written, content)
	return nil
}

type fakeDesktop struct {
	api.Desktop
	clipboard *fakeClipboard
}

func (d fakeDesktop) Clipboard() api.SystemClipboard { return d.clipboard }

type fakeSystem struct {
	api.System
	paste bool
}

func (s fakeSystem) HasCapability(name string) bool { return s.paste && name == "clipboard.paste" }

type fakeClock struct{ api.Clock }

func (fakeClock) Now() time.Time { return testNow }

// fakeNavigation records the views the built-in pushes.
type fakeNavigation struct {
	api.Navigation
	pushed []api.View
}

func (n *fakeNavigation) Push(view *api.View) api.Result {
	n.pushed = append(n.pushed, *view)
	return api.Result{View: view, Navigation: api.NavPush}
}

type fakeViews struct {
	api.Views
	invalidated int
}

func (v *fakeViews) Refresh() api.Action { return api.Run("refresh", "Refresh", nil) }
func (v *fakeViews) InvalidateRoot()     { v.invalidated++ }

// fakeContext is the context a built-in's calls run in, backed by the fakes above.
type fakeContext struct {
	api.Context
	history *fakeHistory
	desktop fakeDesktop
	system  fakeSystem
	nav     *fakeNavigation
	views   *fakeViews
}

func newFakeContext(entries ...api.ClipboardEntry) fakeContext {
	return fakeContext{
		history: &fakeHistory{entries: entries},
		desktop: fakeDesktop{clipboard: &fakeClipboard{}},
		system:  fakeSystem{paste: true},
		nav:     &fakeNavigation{},
		views:   &fakeViews{},
	}
}

func (c fakeContext) Clipboard() api.ClipboardHistory { return c.history }
func (c fakeContext) Desktop() api.Desktop            { return c.desktop }
func (c fakeContext) System() api.System              { return c.system }
func (c fakeContext) Clock() api.Clock                { return fakeClock{} }
func (c fakeContext) Navigation() api.Navigation      { return c.nav }
func (c fakeContext) Views() api.Views                { return c.views }

func text(id, body string, minutesAgo int) api.ClipboardEntry {
	return api.ClipboardEntry{ID: id, Type: kindText, Text: body, CreatedAtMs: testNow.Add(-time.Duration(minutesAgo) * time.Minute).UnixMilli()}
}

func image(id string, minutesAgo int) api.ClipboardEntry {
	return api.ClipboardEntry{
		ID: id, Type: kindImage, ImagePath: "/data/clipboard-images/" + id + ".png", ThumbnailURL: "data:image/png;base64,AAAA",
		CreatedAtMs: testNow.Add(-time.Duration(minutesAgo) * time.Minute).UnixMilli(),
	}
}

func options(list, search bool) func() Options {
	return func() Options { return Options{RootList: list, RootSearch: search, LinkPreviews: true} }
}

func TestManifestIsAValidBuiltIn(t *testing.T) {
	if err := New(nil).Manifest().ValidateBuiltIn(); err != nil {
		t.Fatalf("the built-in manifest is refused: %v", err)
	}
}

func TestCommandIsTheClipboardHistoryRootCommand(t *testing.T) {
	commands, err := New(nil).Commands(newFakeContext())
	if err != nil || len(commands) != 1 {
		t.Fatalf("commands = %v, %v; want one", commands, err)
	}
	if got := commands[0]; got.ID != ListCommandID || got.Title != listTitle || got.Score != scoreRootCommand {
		t.Fatalf("command = %+v", got)
	}
}

func TestRootShowsTheTenNewestCopiesAtScore60(t *testing.T) {
	var entries []api.ClipboardEntry
	for i := range 12 {
		entries = append(entries, text(string(rune('a'+i)), "copy", i))
	}
	rows, err := New(options(true, true)).RootItems(newFakeContext(entries...))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != rootCopyLimit {
		t.Fatalf("root shows %d copies, want %d", len(rows), rootCopyLimit)
	}
	for i, row := range rows {
		if row.Score != scoreRootCopy || row.Section != search.KindClipboard || row.ID != "clipboard:"+entries[i].ID {
			t.Fatalf("row %d = score %v section %q id %q", i, row.Score, row.Section, row.ID)
		}
	}
}

func TestRootHidesCopiesWhenTheRootListIsOff(t *testing.T) {
	rows, _ := New(options(false, true)).RootItems(newFakeContext(text("a", "copy", 1)))
	if len(rows) != 0 {
		t.Fatalf("root list off still shows %d copies", len(rows))
	}
}

func TestSearchFindsTextCopiesOnlyAndScoresThemDown(t *testing.T) {
	c := newFakeContext(text("a", "meeting notes", 1), text("b", "meeting agenda", 2))
	rows, err := New(options(true, true)).SearchItems(c, "meeting")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.history.queries) != 1 || c.history.queries[0].Limit != rootSearchLimit || !hasType(c.history.queries[0].Types, kindHTML) {
		t.Fatalf("search query = %+v, want the text types with limit %d", c.history.queries, rootSearchLimit)
	}
	if len(rows) != 2 || rows[0].Score != scoreSearchCopy || rows[1].Score != scoreSearchCopy-1 {
		t.Fatalf("search rows = %+v", rows)
	}
}

func TestSearchHidesCopiesWhenTheRootSearchIsOff(t *testing.T) {
	rows, _ := New(options(true, false)).SearchItems(newFakeContext(text("a", "meeting", 1)), "meeting")
	if len(rows) != 0 {
		t.Fatalf("root search off still shows %d copies", len(rows))
	}
}

func TestImageRowCarriesItsThumbnailAndDragsItsFile(t *testing.T) {
	row := copyRow(image("shot", 3), 0, rowLook{Now: testNow, CanPaste: true})
	if row.Image == nil || row.Image.Src != "data:image/png;base64,AAAA" {
		t.Fatalf("image row thumbnail = %+v", row.Image)
	}
	if row.File == nil || row.File.Path != "/data/clipboard-images/shot.png" || row.DragImage == nil {
		t.Fatalf("image row drag file = %+v / %+v", row.File, row.DragImage)
	}
	if row.Primary == nil || row.Primary.Kind != api.ActionCopyImage {
		t.Fatalf("image row copy = %+v, want copy-image", row.Primary)
	}
}

func TestTextRowsOfferPasteTextOnlyWhenThePlatformCanPaste(t *testing.T) {
	with := titlesOf(copyRow(text("a", "copy", 1), 0, rowLook{Now: testNow, CanPaste: true}).Actions)
	without := titlesOf(copyRow(text("a", "copy", 1), 0, rowLook{Now: testNow}).Actions)
	if !contains(join(with), "Paste Text") || contains(join(without), "Paste Text") {
		t.Fatalf("actions with paste %v, without %v", with, without)
	}
}

func TestPreviewOpensTheCopyFullPalette(t *testing.T) {
	c := newFakeContext(text("a", "hello", 1))
	if _, err := previewEntry(c, "a"); err != nil {
		t.Fatal(err)
	}
	if len(c.nav.pushed) != 1 || c.nav.pushed[0].Kind != api.ViewPreview || c.nav.pushed[0].Preview.Text != "hello" {
		t.Fatalf("pushed = %+v", c.nav.pushed)
	}
}

func TestCopyHTMLWritesItsMarkup(t *testing.T) {
	c := newFakeContext(api.ClipboardEntry{ID: "h", Type: kindHTML, Text: "bold", HTML: "<b>bold</b>"})
	if _, err := copyEntry(c, "h"); err != nil {
		t.Fatal(err)
	}
	written := c.desktop.clipboard.written
	if len(written) != 1 || written[0].Type != "html" || written[0].HTML != "<b>bold</b>" || written[0].Text != "bold" {
		t.Fatalf("written = %+v", written)
	}
}

func TestRemoveWindowRemovesOnlyTheCopiesInside(t *testing.T) {
	c := newFakeContext(text("new", "a", 2), text("older", "b", 90), text("oldest", "c", 300))
	if _, err := removeWindow(c, windowHour); err != nil {
		t.Fatal(err)
	}
	if got := c.history.removed; len(got) != 1 || len(got[0]) != 1 || got[0][0] != "new" {
		t.Fatalf("removed %v, want only the copy from the last hour", got)
	}
	if c.views.invalidated != 1 {
		t.Fatalf("root invalidations = %d, want 1", c.views.invalidated)
	}
}

func TestRemoveAllClearsTheHistory(t *testing.T) {
	c := newFakeContext(text("a", "x", 1))
	if _, err := removeAll(c); err != nil || !c.history.cleared {
		t.Fatalf("clear called = %v, err %v", c.history.cleared, err)
	}
}

func TestFailedListIsReturnedNotHidden(t *testing.T) {
	c := newFakeContext()
	c.history.failList = errors.New("history locked")
	if _, err := New(options(true, true)).RootItems(c); err == nil {
		t.Fatal("a failed list was hidden")
	}
}

func TestSingleWebAddressGetsALinkCard(t *testing.T) {
	if got := singleWebURL("https://example.com/a"); got != "https://example.com/a" {
		t.Fatalf("single address = %q", got)
	}
	if singleWebURL("see https://example.com") != "" || singleWebURL("mailto:a@b.c") != "" {
		t.Fatal("a sentence or another scheme got a link card")
	}
}

func TestRemovedMessageCountsItems(t *testing.T) {
	if removedMessage(1) != "Removed 1 clipboard item." || removedMessage(3) != "Removed 3 clipboard items." {
		t.Fatal("removed messages")
	}
}

func titlesOf(actions []api.Action) []string {
	out := make([]string, 0, len(actions))
	for _, a := range actions {
		out = append(out, a.Title)
	}
	return out
}

func join(parts []string) string {
	out := ""
	for _, p := range parts {
		out += p + "\n"
	}
	return out
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func hasType(types []string, want string) bool {
	for _, t := range types {
		if t == want {
			return true
		}
	}
	return false
}

// TestListMenuGroupsTheRemovalsUnderManageHistory checks the list's own menu: the removals sit in the "Manage History" group,
// as 1.x titles them.
func TestListMenuGroupsTheRemovalsUnderManageHistory(t *testing.T) {
	read := options(true, true)
	view := New(read).listView(newFakeContext(text("a", "copy", 1)), read())
	if view.ActionPanel == nil || len(view.ActionPanel.Sections) != 1 || view.ActionPanel.Sections[0].Title != "Manage History" {
		t.Fatalf("the list menu sections = %+v, want one group titled Manage History", view.ActionPanel)
	}
	if n := len(view.ActionPanel.Sections[0].Actions); n != 3 {
		t.Fatalf("the Manage History group holds %d removals, want 3", n)
	}
}
