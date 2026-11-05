package files

import (
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
	argQuery         = "query"
	searchTitle      = "Search Files"
)

// searchFilesView is the Search Files view: the recent files until the user types, then the index matches of the typed
// name, filtered by the kind the search accessory holds.
func (e Extension) searchFilesView(ctx api.Context) api.View {
	e.state.reset()
	recent := withThumbnails(ctx, e.rowsFor(ctx, "", "", viewLimit))
	view := api.ListView(searchTitle, recent...)
	view.Placeholder = searchTitle
	view.EmptyView = &api.EmptyState{Title: "No files found.", Subtitle: "Type a file name. Use the kind filter to narrow the results."}
	view.OnSearchChange = &api.ActionRef{ID: refSearch}
	view.SearchAccessory = &api.SearchAccessory{ID: "kind", Tooltip: "Kind", Value: "", Options: searchKinds, OnChange: &api.ActionRef{ID: refKind}}
	return view
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

// kindChanged answers a new kind from the search accessory, for the text last searched.
func (e Extension) kindChanged(ctx api.Context, kind string) api.Result {
	query := e.state.setKind(kind)
	items := withThumbnails(ctx, e.rowsFor(ctx, query, kind, viewLimit))
	return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: items, SearchResultIds: itemIDs(items)}}
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
