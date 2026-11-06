// Package extview draws the host-rendered views an extension returns (api.View) with the palette's native widgets.
// A Session is one open view: it owns the view's items, the Markdown options its drawings keep between frames, the
// handles of the actions it issued, and the loads, refreshes and images it started. Extension code never reaches the
// palette directly; it answers through Deps, and the palette answers back through handles.
//
// Layer: palette. The package imports api, search, palette, design, platform and mygo's ui.
package extview

import (
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/aichat"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Session is one open extension view. It is used on the UI thread only.
type Session struct {
	deps         Deps
	chatRenderer *aichat.ExtensionChat
	// chatSideSession and chatSidePage are the side page of a chat view, kept between renders.
	chatSideSession *Session
	chatSidePage    *palette.View
	viewIntent      bool
	extID           string
	view            api.View
	host            search.Host
	items           []api.Item
	sections        []api.ItemSection
	loading         bool
	failure         *api.Failure
	hasMore         bool
	selectID        string
	images          map[string]*ui.Bitmap
	requested       map[string]bool
	handles         map[handle]route
	markdown        *markdown.Options
	started         bool
	immediate       bool
	dismissed       bool
	refreshing      bool
	refreshPending  bool
	itemsRevision   uint64
	stopInterval    chan struct{}
	form            *formState
	media           *palette.View
	page            platform.WebPage
	camera          *cameraView
	searchChange    searchChange
	accessory       *palette.Accessory
	// guard covers the maps a row builds on a search goroutine: the run handles and the images.
	guard        sync.Mutex
	filePreviews map[string]filePreviewText
}

// NewSession prepares view for the extension extID. Nothing runs until Open shows it.
func NewSession(deps Deps, extID string, view api.View) *Session {
	if deps.Cameras == nil {
		deps.Cameras = NewMemoryCameras()
	}
	s := &Session{
		deps: deps, extID: extID, view: view, loading: view.IsLoading, viewIntent: true,
		items: view.Items, sections: view.Sections, selectID: view.SelectedItemID,
		images: map[string]*ui.Bitmap{}, requested: map[string]bool{},
		handles: map[handle]route{}, markdown: &markdown.Options{},
	}
	s.hasMore = view.Pagination != nil && view.Pagination.HasMore && view.Pagination.Next != nil
	return s
}

// Open shows the session on host: it starts the view's loads and refreshes, and returns the page to push.
func (s *Session) Open(host search.Host) *palette.View {
	s.host = host
	if s.isMediaView() {
		s.openMedia()
		return s.media
	}
	s.start()
	return s.Palette()
}

// Palette returns the palette page that draws the view. The page reads the session on every search, so patches show
// on the next frame after the host refreshes it.
func (s *Session) Palette() *palette.View {
	page := s.pageOfKind()
	page.Subtitle = s.view.Subtitle
	if s.hasPageActions() {
		page.Actions = s.pageActions
	}
	if len(s.view.FooterHints) > 0 {
		page.Hint = footerHintsText(s.view.FooterHints, s.hyperKey())
		page.HintReplacesActions = true
	}
	return page
}

// hasPageActions is true when the view's own menu lists an action, so its shortcuts run from any page of the view.
func (s *Session) hasPageActions() bool {
	return len(MenuWireActions(nil, s.view.ActionPanel, s.view.Actions)) > 0
}

// pageOfKind builds the palette page of the view's kind.
func (s *Session) pageOfKind() *palette.View {
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
	case api.ViewForm:
		if s.view.Form != nil {
			return s.formPage()
		}
	case api.ViewEditor:
		if s.view.Editor != nil {
			return s.editorView(*s.view.Editor)
		}
	case api.ViewWebview, api.ViewCamera:
		if s.media != nil {
			return s.media
		}
	case api.ViewSettings:
		if s.view.Settings != nil {
			return s.settingsPage()
		}
	case api.ViewChat:
		if s.view.Chat != nil {
			return s.chatPage()
		}
	}
	return s.unsupportedPage()
}

// listPage is the searchable list of items, with its search accessory, its loading state and its close hook.
func (s *Session) listPage() *palette.View {
	page := &palette.View{Title: s.view.Title, Placeholder: s.view.Placeholder, Presentation: palettePresentation(s.view.Presentation), Hint: s.view.Hint}
	if s.view.Search != nil {
		page.Providers = s.searchProviders
	} else {
		page.Search = func(q string) []search.Item {
			page.SelectID = s.takeSelectID()
			s.scheduleSearchChange(q)
			return s.rows(q)
		}
	}
	page.Loading = s.isLoading
	page.Pending = s.searchChangePending
	page.EmptyTitle, page.EmptySub = s.emptyTitle(), s.emptySubtitle()
	page.EmptyFn = s.emptyState
	page.OnLeave = s.close
	if s.view.Refresh != nil && s.view.Refresh.Ref != nil {
		page.OnReveal = s.refreshNow
	}
	page.Accessory = s.searchAccessory()
	page.Actions = s.pageActions
	page.OnKey = s.tabCyclesAccessory(page.Accessory)
	page.Footer = s.footerName
	page.Hint = s.view.Hint
	return page
}

// emptyState is the empty state of a list as the view says it now: a refresh can change it while the view is open.
func (s *Session) emptyState() (string, string) {
	return s.emptyTitle(), s.emptySubtitle()
}

// palettePresentation maps a view's presentation to the palette's. A side-preview list shows the detail of the chosen row
// beside its rows, and the palette sizes the window for it; a preview view takes the preview size; the rest stack.
func palettePresentation(presentation api.Presentation) palette.Presentation {
	switch presentation {
	case api.PresentSidePreview:
		return palette.SidePreview
	case api.PresentPreview:
		return palette.Preview
	}
	return palette.Stacked
}

func (s *Session) gridPage() *palette.View {
	page := s.listPage()
	page.Grid = s.gridSpecOf()
	if s.view.Grid != nil && s.view.Grid.CategoryKeys {
		page.OnKey = s.categoryKeys
	}
	if s.view.Grid != nil && s.view.Grid.SelectionFooter {
		page.Footer = selectionFooter
	}
	page.Limit = s.view.MaxVisibleItems
	return page
}

// searchAccessory is the picker at the end of the search field, when the view names options and a handler.
func (s *Session) searchAccessory() *palette.Accessory {
	spec := s.view.SearchAccessory
	if spec == nil || spec.OnChange == nil || len(spec.Options) == 0 {
		return nil
	}
	options := make([]palette.AccessoryOption, 0, len(spec.Options))
	for _, option := range spec.Options {
		options = append(options, palette.AccessoryOption{Title: option.Title, Value: option.Value})
	}
	accessory := &palette.Accessory{Value: spec.Value, Options: options}
	accessory.OnChange = func(value string) { s.invokeInPlace(*spec.OnChange, value) }
	s.accessory = accessory
	return accessory
}

func (s *Session) takeSelectID() string {
	id := s.selectID
	s.selectID = ""
	return id
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

// rows answers a query: the failure guidance when a load failed, then every matching item grouped by the view's
// sections, then the load-more row while the view has pages left.
func (s *Session) rows(query string) []search.Item {
	var out []search.Item
	if s.failure != nil {
		out = append(out, s.failureRows()...)
	}
	if s.loading && len(s.items) == 0 && len(s.sections) == 0 {
		return out
	}
	if len(s.sections) > 0 {
		for _, section := range s.sections {
			out = s.appendSectionRows(out, section.Title, section.Items, query)
		}
	} else {
		out = s.appendSectionRows(out, "", s.items, query)
	}
	out = s.withSecondBatch(out, query)
	if s.hasMore {
		out = append(out, s.loadMoreRow())
	}
	return out
}

// withSecondBatch appends the handler's second batch after the local matches. As 1.x does, the batch takes its room
// from MaxVisibleItems first, so a long local list cannot push it out of view.
func (s *Session) withSecondBatch(local []search.Item, query string) []search.Item {
	extra := s.searchChangeRows(local, query)
	if len(extra) == 0 {
		return local
	}
	if limit := s.view.MaxVisibleItems; limit > 0 {
		local = local[:min(len(local), max(0, limit-len(extra)))]
	}
	return append(local, extra...)
}

func (s *Session) appendSectionRows(out []search.Item, title string, items []api.Item, query string) []search.Item {
	first := true
	for _, item := range matchingItems(items, query) {
		row := s.row(item)
		if first && title != "" {
			row.Section = title
		}
		first = false
		out = append(out, row)
	}
	return out
}

// isLoading reports both cold loads and refreshes; the host preserves cached rows during refresh.
func (s *Session) isLoading() bool {
	return s.loading
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

// runDismissal says whether the palette stays open while an ActionRun handler works. An action that asks to be
// dismissed after its run closes the palette as the run is dispatched, as the 1.x auto dismissal did.
func runDismissal(action api.Action) search.Dismissal {
	if action.DismissAfterRun {
		return search.DismissAuto
	}
	return search.DismissNever
}
