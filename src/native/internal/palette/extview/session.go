// Package extview draws the host-rendered views an extension returns (api.View) with the palette's native widgets.
// A Session is one open view: it owns the view's items, the Markdown options its drawings keep between frames, and
// the handles of the actions it issued. Extension code never reaches the palette directly; it answers through Deps,
// and the palette answers back through handles.
//
// Layer: palette. The package imports api, search, palette, design, platform and mygo's ui.
package extview

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Deps are the services a view uses. Invoke runs an ActionRun of an extension off the UI thread and calls done on the
// UI thread with its result; the host owns the timeout and the process. The other ports are declarative host operations.
type Deps struct {
	Invoke   func(extID string, in api.Invocation, done func(api.Result, *api.Failure))
	OpenURL  func(url string)
	CopyText func(text string)
	Files    platform.FS
}

// Session is one open extension view. It is used on the UI thread only.
type Session struct {
	deps     Deps
	extID    string
	view     api.View
	items    []api.Item
	sections []api.ItemSection
	loading  bool
	handles  map[handle]route
	markdown *markdown.Options
}

// NewSession opens view for the extension extID. Its items are the view's own; patches change them later.
func NewSession(deps Deps, extID string, view api.View) *Session {
	return &Session{
		deps: deps, extID: extID, view: view,
		items: view.Items, sections: view.Sections, loading: view.IsLoading,
		handles: map[handle]route{}, markdown: &markdown.Options{},
	}
}

// Palette returns the palette page that draws the view. The page reads the session on every search, so patches show
// on the next frame after the host refreshes it.
func (s *Session) Palette() *palette.View {
	switch s.view.Kind {
	case api.ViewList:
		return s.listPage()
	case api.ViewGrid:
		return s.gridPage()
	case api.ViewCollection:
		return s.collectionPage()
	case api.ViewPreview:
		return s.previewPage()
	case api.ViewProgress:
		return s.progressPage()
	case api.ViewFilePreview:
		return s.filePreviewPage()
	}
	return s.unsupportedPage()
}

// listPage is the searchable list of items. Its rows follow the session's items each time the palette searches.
func (s *Session) listPage() *palette.View {
	return &palette.View{
		Title: s.view.Title, Placeholder: s.view.Placeholder,
		Search:     func(q string) []search.Item { return s.rows(q) },
		EmptyTitle: s.emptyTitle(), EmptySub: s.emptySubtitle(),
	}
}

func (s *Session) gridPage() *palette.View {
	page := s.listPage()
	page.Grid = &palette.Grid{Columns: s.gridColumns(), CellHeight: gridCellHeight, Cell: drawGridCell}
	return page
}

func (s *Session) emptyTitle() string {
	if s.view.EmptyView != nil && s.view.EmptyView.Title != "" {
		return s.view.EmptyView.Title
	}
	return "No items"
}

func (s *Session) emptySubtitle() string {
	if s.view.EmptyView != nil {
		return s.view.EmptyView.Subtitle
	}
	return ""
}

// rows answers a query: every item that matches it, grouped by the view's sections, in the order the extension sent.
func (s *Session) rows(query string) []search.Item {
	if s.loading && len(s.items) == 0 && len(s.sections) == 0 {
		return nil
	}
	var out []search.Item
	if len(s.sections) > 0 {
		for _, section := range s.sections {
			out = appendSectionRows(out, s, section.Title, section.Items, query)
		}
		return out
	}
	return appendSectionRows(out, s, "", s.items, query)
}

func appendSectionRows(out []search.Item, s *Session, title string, items []api.Item, query string) []search.Item {
	first := true
	for _, item := range items {
		if !matchesQuery(item, query) {
			continue
		}
		row := s.row(item)
		if first && title != "" {
			row.Section = title
		}
		first = false
		out = append(out, row)
	}
	return out
}

// unsupportedPage says, in the palette, that this build cannot draw the view kind yet. It never fails silently.
func (s *Session) unsupportedPage() *palette.View {
	return &palette.View{
		Title: s.view.Title, HideInput: true,
		Body: func(c *ui.Context, _ search.Host) {
			drawNotice(c, "This view type is not available in this build yet.")
		},
	}
}
