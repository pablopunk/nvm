package extview

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fakeHost is the palette as an action sees it: it records what the action asked for.
type fakeHost struct {
	hidden  bool
	pops    int
	pushed  []*palette.View
	toasts  []string
	refresh int
}

func (h *fakeHost) Hide()            { h.hidden = true }
func (h *fakeHost) Pop()             { h.pops++ }
func (h *fakeHost) Push(view any)    { h.pushed = append(h.pushed, view.(*palette.View)) }
func (h *fakeHost) Toast(msg string) { h.toasts = append(h.toasts, msg) }
func (h *fakeHost) Refresh()         { h.refresh++ }
func (h *fakeHost) Query() string    { return "" }
func (h *fakeHost) SetQuery(string)  {}

var _ search.Host = (*fakeHost)(nil)

// invokeLog records the invocations an extension received and answers each with the canned result.
type invokeLog struct {
	calls   []string
	results map[string]api.Result
	failure *api.Failure
}

func (l *invokeLog) deps(files *fake.FS) Deps {
	return Deps{
		Invoke: func(extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			l.calls = append(l.calls, extID+":"+in.Ref.ID)
			done(l.results[in.Ref.ID], l.failure)
		},
		OpenURL:  func(string) {},
		CopyText: func(search.Host, string) {},
		Files:    files,
	}
}

func listItems(items ...api.Item) api.View {
	return api.View{Kind: api.ViewList, Title: "Notes", Items: items}
}

func rowTitles(rows []search.Item) []string {
	titles := make([]string, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, row.Title)
	}
	return titles
}

func findRow(t *testing.T, rows []search.Item, title string) search.Item {
	t.Helper()
	for _, row := range rows {
		if row.Title == title {
			return row
		}
	}
	t.Fatalf("no row %q among %v", title, rowTitles(rows))
	return search.Item{}
}

func actionNamed(t *testing.T, row search.Item, title string) search.Action {
	t.Helper()
	for _, action := range row.Actions {
		if action.Title == title {
			return action
		}
	}
	t.Fatalf("row %q has no action %q", row.Title, title)
	return search.Action{}
}

func TestListRowsCarryTitleAccessoryDetailAndPrimaryFirst(t *testing.T) {
	view := listItems(api.Item{
		ID: "a", Title: "Alpha", Subtitle: "Draft", Icon: "file-text",
		Accessories: []api.Accessory{{Text: "Draft"}, {Text: "2 KB"}},
		Detail:      &api.Detail{Title: "Alpha", Markdown: "# Alpha"},
		Primary:     &api.Action{Kind: api.ActionCopyText, Title: "Copy", Text: "Alpha"},
		Actions:     []api.Action{{Kind: api.ActionCopyText, Title: "Copy", Text: "Alpha"}, {Kind: api.ActionOpenURL, Title: "Open", URL: "https://example.com"}},
	})
	s := NewSession(Deps{}, "acme", view)
	row := findRow(t, s.rows(""), "Alpha")
	if row.Accessory != "Draft +1" || row.Subtitle != "Draft" || row.Icon != "file-text" {
		t.Fatalf("row = accessory %q subtitle %q icon %q", row.Accessory, row.Subtitle, row.Icon)
	}
	if row.Detail == nil {
		t.Fatal("row has no detail pane")
	}
	if got := []string{row.Actions[0].Title, row.Actions[1].Title}; got[0] != "Copy" || got[1] != "Open" {
		t.Fatalf("actions = %v, want the primary Copy once, then Open", got)
	}
	if len(row.Actions) != 3 || row.Actions[2].Title != api.PreviewTitle {
		t.Fatalf("the primary is listed twice or Preview is missing: %d actions", len(row.Actions))
	}
}

func TestQueryKeepsOnlyMatchingItems(t *testing.T) {
	s := NewSession(Deps{}, "acme", listItems(
		api.Item{ID: "a", Title: "Alpha note"},
		api.Item{ID: "b", Title: "Beta task", Keywords: []string{"todo"}},
	))
	if got := rowTitles(s.rows("beta")); len(got) != 1 || got[0] != "Beta task" {
		t.Fatalf("query beta = %v", got)
	}
	if got := rowTitles(s.rows("todo")); len(got) != 1 || got[0] != "Beta task" {
		t.Fatalf("keyword todo = %v", got)
	}
	if got := rowTitles(s.rows("")); len(got) != 2 {
		t.Fatalf("empty query = %v, want every item", got)
	}
}

func TestSectionsGroupRowsUnderTheirTitles(t *testing.T) {
	s := NewSession(Deps{}, "acme", api.View{Kind: api.ViewList, Title: "Tasks", Sections: []api.ItemSection{
		{Title: "Open", Items: []api.Item{{ID: "t1", Title: "Write"}, {ID: "t2", Title: "Review"}}},
		{Title: "Done", Items: []api.Item{{ID: "t3", Title: "Send"}}},
	}})
	rows := s.rows("")
	if rows[0].Section != "Open" || rows[1].Section != "" || rows[2].Section != "Done" {
		t.Fatalf("sections = %q %q %q", rows[0].Section, rows[1].Section, rows[2].Section)
	}
}

func TestPatchMergesByIDAndDropsUnknownIDs(t *testing.T) {
	s := NewSession(Deps{}, "acme", listItems(api.Item{ID: "a", Title: "Alpha"}, api.Item{ID: "b", Title: "Beta"}))
	s.Apply(api.ViewPatch{Items: []api.Item{{ID: "a", Title: "Alpha done"}, {ID: "zzz", Title: "Unknown"}}})
	if got := rowTitles(s.rows("")); len(got) != 2 || got[0] != "Alpha done" || got[1] != "Beta" {
		t.Fatalf("merged rows = %v", got)
	}
}

func TestPatchModesReplacePrependAppendAndRemove(t *testing.T) {
	base := []api.Item{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}, {ID: "c", Title: "C"}}
	tests := []struct {
		name  string
		patch api.ViewPatch
		want  []string
	}{
		{"replace", api.ViewPatch{Mode: api.PatchReplace, Items: []api.Item{{ID: "x", Title: "X"}}}, []string{"X"}},
		{"prepend", api.ViewPatch{Mode: api.PatchPrepend, Items: []api.Item{{ID: "x", Title: "X"}}}, []string{"X", "A", "B", "C"}},
		{"append", api.ViewPatch{Mode: api.PatchAppend, Items: []api.Item{{ID: "b", Title: "B2"}}}, []string{"A", "C", "B2"}},
		{"remove first", api.ViewPatch{RemoveItemIDs: []string{"a"}, Items: []api.Item{{ID: "d", Title: "D"}}}, []string{"B", "C"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession(Deps{}, "acme", listItems(base...))
			s.Apply(tt.patch)
			if got := rowTitles(s.rows("")); strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("rows = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPatchIsLoadingSetsTheLoadingState(t *testing.T) {
	s := NewSession(Deps{}, "acme", listItems())
	loading := true
	s.Apply(api.ViewPatch{IsLoading: &loading})
	if !s.loading {
		t.Fatal("isLoading patch did not set the loading state")
	}
}

func TestRunActionPatchesTheVisibleRowsAndRefreshes(t *testing.T) {
	log := &invokeLog{results: map[string]api.Result{
		"reviewed": {Patch: &api.ViewPatch{Items: []api.Item{{ID: "a", Title: "Alpha reviewed"}}}},
	}}
	view := listItems(api.Item{ID: "a", Title: "Alpha", Actions: []api.Action{api.Run("reviewed", "Review", nil)}})
	s := NewSession(log.deps(nil), "acme", view)
	h := &fakeHost{}
	actionNamed(t, findRow(t, s.rows(""), "Alpha"), "Review").Run(h)

	if want := []string{"acme:reviewed"}; strings.Join(log.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v, want %v", log.calls, want)
	}
	if h.refresh != 1 || h.hidden {
		t.Fatalf("patch result: refresh %d hidden %v, want one refresh and the palette open", h.refresh, h.hidden)
	}
	if got := rowTitles(s.rows("")); got[0] != "Alpha reviewed" {
		t.Fatalf("rows after the patch = %v", got)
	}
}

func TestRunActionViewOpensAboveTheCurrentPage(t *testing.T) {
	log := &invokeLog{results: map[string]api.Result{"open": {View: &api.View{Kind: api.ViewPreview, Title: "Next", Preview: &api.PreviewSpec{Text: "hi"}}}}}
	s := NewSession(log.deps(nil), "acme", listItems(api.Item{ID: "a", Title: "Alpha", Primary: ptrAction(api.Run("open", "Open", nil))}))
	h := &fakeHost{}
	findRow(t, s.rows(""), "Alpha").Actions[0].Run(h)
	if len(h.pushed) != 1 || h.hidden {
		t.Fatalf("view result: pushed %d hidden %v, want one pushed page with the palette open", len(h.pushed), h.hidden)
	}
}

func TestEmptyResultClosesThePalette(t *testing.T) {
	log := &invokeLog{}
	s := NewSession(log.deps(nil), "acme", listItems(api.Item{ID: "a", Title: "Alpha", Primary: ptrAction(api.Run("done", "Run", nil))}))
	h := &fakeHost{}
	findRow(t, s.rows(""), "Alpha").Actions[0].Run(h)
	if !h.hidden {
		t.Fatal("an empty result left the palette open")
	}
}

func TestFailureShowsItsMessageAndKeepsThePaletteOpen(t *testing.T) {
	log := &invokeLog{failure: &api.Failure{Code: api.CodeTimeout, Message: "the extension did not answer in time"}}
	s := NewSession(log.deps(nil), "acme", listItems(api.Item{ID: "a", Title: "Alpha", Primary: ptrAction(api.Run("slow", "Run", nil))}))
	h := &fakeHost{}
	findRow(t, s.rows(""), "Alpha").Actions[0].Run(h)
	if h.hidden || len(h.toasts) != 1 || h.toasts[0] != "the extension did not answer in time" {
		t.Fatalf("failure: hidden %v toasts %v", h.hidden, h.toasts)
	}
}

func TestDestructiveActionAsksBeforeItRuns(t *testing.T) {
	log := &invokeLog{results: map[string]api.Result{"remove": {}}}
	remove := api.Run("remove", "Remove", nil)
	remove.Confirm = &api.Confirmation{Title: "Remove “Alpha”?", ConfirmLabel: "Remove", Destructive: true}
	s := NewSession(log.deps(nil), "acme", listItems(api.Item{ID: "a", Title: "Alpha", Primary: ptrAction(remove)}))

	h := &fakeHost{}
	findRow(t, s.rows(""), "Alpha").Actions[0].Run(h)
	if len(log.calls) != 0 || len(h.pushed) != 1 {
		t.Fatalf("before confirming: calls %v pushed %d, want no call and one confirmation page", log.calls, len(h.pushed))
	}
	confirm := h.pushed[0].Search("")
	confirm[0].Actions[0].Run(h)
	if want := []string{"acme:remove"}; strings.Join(log.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("after confirming: calls = %v", log.calls)
	}
}

func TestStaleHandleIsRefused(t *testing.T) {
	log := &invokeLog{}
	s := NewSession(log.deps(nil), "acme", listItems(api.Item{ID: "a", Title: "Alpha", Primary: ptrAction(api.Run("x", "Run", nil))}))
	row := findRow(t, s.rows(""), "Alpha")
	s.handles = map[handle]route{}
	h := &fakeHost{}
	row.Actions[0].Run(h)
	if len(log.calls) != 0 || len(h.toasts) != 1 {
		t.Fatalf("stale handle ran: calls %v toasts %v", log.calls, h.toasts)
	}
}

func TestCollectionAddsRemovesAndPreviewsRecords(t *testing.T) {
	remove := api.Action{Kind: api.ActionRun, Title: "Remove", Ref: &api.ActionRef{ID: "remove"}}
	view := api.CollectionView("Snippets", &api.Action{Kind: api.ActionRun, Title: "New", Ref: &api.ActionRef{ID: "create"}},
		api.CrudItem{ID: "n1", Title: "Standup", Preview: &api.Action{Kind: api.ActionRun, Title: "Preview", Ref: &api.ActionRef{ID: "preview"}}, Remove: &remove})
	s := NewSession(Deps{}, "acme", view)
	page := s.Palette()
	rows := page.Search("")
	if rows[0].ID != collectionAddID || rows[0].Title != "New" {
		t.Fatalf("first row = %q %q, want the add row", rows[0].ID, rows[0].Title)
	}
	record := findRow(t, rows, "Standup")
	if record.Actions[0].Title != "Preview" {
		t.Fatalf("record primary = %q, want Preview", record.Actions[0].Title)
	}
	if got := actionNamed(t, record, "Remove"); got.Title != "Remove" {
		t.Fatalf("remove action missing")
	}
}

func TestCollectionRemoveConfirmsByDefault(t *testing.T) {
	item := collectionItem(api.CrudItem{ID: "n1", Title: "Standup", Remove: &api.Action{Kind: api.ActionRun, Title: "Remove", Ref: &api.ActionRef{ID: "remove"}}})
	if item.Primary == nil || item.Primary.Title != "Remove" || item.Primary.Confirm == nil || !item.Primary.Confirm.Destructive {
		t.Fatalf("remove-only record = %+v, want a destructive confirmed Remove as its primary", item.Primary)
	}
}

func TestUnsupportedViewKindSaysSo(t *testing.T) {
	page := NewSession(Deps{}, "acme", api.View{Kind: api.ViewWebview, Title: "Web"}).Palette()
	if page.Body == nil {
		t.Fatal("unsupported view has no body to say why it cannot draw")
	}
}

func ptrAction(a api.Action) *api.Action { return &a }

// Root has nothing to leave in this test host.
func (h *fakeHost) Root() {}
