package files

import (
	"strconv"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// The Search Files view's kinds, offered by its search accessory. "All files" is the empty kind.
var searchKinds = []api.SearchAccessoryOption{
	{Title: "All files", Value: ""},
	{Title: "Images", Value: "image"},
	{Title: "Videos", Value: "video"},
	{Title: "Media", Value: "media"},
}

// Refs of the view actions. The search and kind refs run on each keystroke pause and kind change of the Search Files
// view; the meaning refs drive Find Files by Meaning.
const (
	refSearch        = "files-search"
	refKind          = "files-kind"
	refOpenSearch    = "search-files"
	refMeaningSearch = "meaning-search"
	refMeaningAsk    = "meaning-ask"
	refOpenMeaning   = "find-files-by-meaning"
	refStatus        = "files-status"
	argQuery         = "query"
	searchTitle      = "Search Files"
	searchHint       = "Tab kind"
	searchEmptySub   = "Type a file name. Press Tab to change the kind."
	statusIntervalMs = 1000
	indexingTitle    = "Indexing…"
	noFilesTitle     = "No files found."
)

// searchFilesView is the Search Files view: the recent files until the user types, then the index matches of the typed
// name, filtered by the kind the search accessory holds. Tab steps through the kinds, and the footer shows the kind and
// the index's count, refreshed while the index fills.
func (e Extension) searchFilesView(ctx api.Context) api.View {
	e.state.reset()
	recent := withThumbnails(ctx, e.rowsFor(ctx, "", "", viewLimit))
	view := api.ListView(searchTitle, recent...)
	view.Placeholder = searchTitle
	view.Hint = searchHint
	view.Status = statusText(ctx, "")
	view.EmptyView = indexEmptyState(ctx)
	view.Refresh = &api.Refresh{IntervalMs: statusIntervalMs, Ref: &api.ActionRef{ID: refStatus}}
	view.OnSearchChange = &api.ActionRef{ID: refSearch}
	view.SearchAccessory = &api.SearchAccessory{ID: "kind", Tooltip: "Kind", Value: "", Options: searchKinds, OnChange: &api.ActionRef{ID: refKind}, CycleOnTab: true}
	return view
}

// statusText is the footer of the Search Files view for a kind: the kind's title, then the index's count.
func statusText(ctx api.Context, kind string) string {
	return kindTitle(kind) + " · " + indexLabel(ctx.Desktop().Files().IndexStatus())
}

// kindTitle is the title of a kind in the search accessory, or "All files" for an unknown one.
func kindTitle(kind string) string {
	for _, option := range searchKinds {
		if option.Value == kind {
			return option.Title
		}
	}
	return searchKinds[0].Title
}

// indexLabel describes the index in the words of the footer: indexing until it holds a snapshot, then its count.
func indexLabel(st api.IndexStatus) string {
	switch {
	case !st.Ready:
		return indexingTitle
	case st.Scanning:
		return "Indexing… " + strconv.FormatInt(st.ScannedFiles, 10) + " files"
	default:
		return strconv.Itoa(st.Entries) + " files indexed"
	}
}

// indexEmptyState is the empty state of the Search Files view: while the first index loads it says so, instead of
// claiming there are no files.
func indexEmptyState(ctx api.Context) *api.EmptyState {
	title := noFilesTitle
	if !ctx.Desktop().Files().IndexStatus().Ready {
		title = indexingTitle
	}
	return &api.EmptyState{Title: title, Subtitle: searchEmptySub}
}

// statusRefresh answers the view's refresh: the footer's count and the empty state, as the index stands now.
func (e Extension) statusRefresh(ctx api.Context) api.Result {
	return api.Result{Patch: &api.ViewPatch{Status: statusText(ctx, e.state.currentKind()), EmptyView: indexEmptyState(ctx)}}
}

// rowsFor are the view rows of a query and kind: the recent files for the empty query, the name matches otherwise.
func (e Extension) rowsFor(ctx api.Context, query, kind string, limit int) []api.Item {
	var found []api.FileRef
	if query == "" {
		found = recentFiles(ctx, limit, kind)
	} else {
		found = searchFiles(ctx, limit, query, kind)
	}
	items := make([]api.Item, 0, len(found))
	for i, file := range found {
		items = append(items, fileItem(ctx, file, float64(limit-i)))
	}
	return items
}

// searchChanged answers a new search text: the rows replace the list, and the host shows the new batch after the
// local matches.
func (e Extension) searchChanged(ctx api.Context, query string) api.Result {
	kind := e.state.currentKind()
	e.state.setQuery(query)
	items := withThumbnails(ctx, e.rowsFor(ctx, query, kind, viewLimit))
	return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: items, SearchResultIds: itemIDs(items)}}
}

// kindChanged answers a new kind from the search accessory, for the text last searched, and shows the kind in the footer.
func (e Extension) kindChanged(ctx api.Context, kind string) api.Result {
	query := e.state.setKind(kind)
	items := withThumbnails(ctx, e.rowsFor(ctx, query, kind, viewLimit))
	return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: items, SearchResultIds: itemIDs(items), Status: statusText(ctx, kind)}}
}

func itemIDs(items []api.Item) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

// reset forgets the query and kind, as a view opened by navigation starts without them.
func (s *viewState) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.query, s.kind, s.askedQuery, s.rows = "", "", "", nil
}

func (s *viewState) currentKind() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kind
}

func (s *viewState) setQuery(query string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.query = query
}

// setKind keeps the kind and returns the query last searched, which the kind change searches again.
func (s *viewState) setKind(kind string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kind = kind
	return s.query
}
