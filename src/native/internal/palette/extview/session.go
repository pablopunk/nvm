// Package extview draws the host-rendered views an extension returns (api.View) with the palette's native widgets.
// A Session is one open view: it owns the view's items, the Markdown options its drawings keep between frames, the
// handles of the actions it issued, and the loads, refreshes and images it started. Extension code never reaches the
// palette directly; it answers through Deps, and the palette answers back through handles.
//
// Layer: palette. The package imports api, search, palette, design, platform and mygo's ui.
package extview

import (
	"encoding/json"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Deps are the services a view uses. Each asynchronous one calls its done function on the UI thread through OnUI; the
// host owns the timeouts and the processes. A nil Clock or Cache disables snapshots, and a nil OnUI runs callbacks in
// place (tests).
type Deps struct {
	Invoke    func(extID string, in api.Invocation, done func(api.Result, *api.Failure))
	Load      func(extID, loader string, done func([]api.Item, *api.Failure))
	LoadImage func(ref string, done func(*ui.Bitmap))
	OpenURL   func(url string)
	// CopyText writes text to the clipboard and reports a failure as a toast on h. Paste sends a paste or typing action to
	// the app in front, through the services of extension extID, and hides the palette first. Nil makes those actions
	// report unavailable.
	CopyText func(h search.Host, text string)
	Paste    func(h search.Host, extID string, action api.Action)
	// OpenSettingsPane opens the operating system settings pane with the id, off the UI thread. Nil makes the
	// system action report unavailable.
	OpenSettingsPane func(id string)
	// RecordPaletteShortcut pushes the recorder of the palette shortcut onto h. Nil makes the action report unavailable.
	RecordPaletteShortcut func(h search.Host)
	// ReadSetting returns the stored value of a declared setting of extension extID, and WriteSetting saves one (both
	// through the extension's settings service). RecordSetting pushes the host's recorder for a setting that holds an
	// accelerator, and RowAccessory gives the host's edge text for a settings row, when it has one. Nil makes the
	// settings rows report unavailable, or show their own text.
	ReadSetting   func(extID, key string) (json.RawMessage, error)
	WriteSetting  func(extID, key string, value json.RawMessage) error
	RecordSetting func(h search.Host, extID, key string)
	RowAccessory  func(extID, rowID string) (string, bool)
	// SignIn runs the host's sign-in flow for the palette's account, on h. Nil makes the account action report unavailable.
	SignIn func(h search.Host)
	// OpenPath opens a file or folder with its default app, and RevealPath selects it in the file manager. Nil makes
	// the action report unavailable.
	OpenPath   func(path string)
	RevealPath func(path string)
	// Indicate shows a result's toast as a passive indicator owned by the extension. Nil shows the message under the
	// search field instead.
	Indicate func(extID string, input api.IndicatorInput)
	// RunSystem runs an OS command by its op, off the UI thread. done receives the sentence to show when the OS did not
	// run it, and "" when it did. Nil makes system ops report unavailable.
	RunSystem func(op string, done func(failure string))
	Files     platform.FS
	Clock     platform.Clock
	Cache     Cache
	OnUI      func(fn func())
	// HyperKey is the Hyper key that shortcut labels show as ✦. It is asked on each label. Nil shows none.
	HyperKey func() string
	// Dialog shows the native file and folder choosers for path fields. Nil makes those fields report unavailable.
	Dialog platform.FileDialog
	// Drafts keeps the unsaved text of editors. Nil keeps editors without autosave.
	Drafts DraftStore
	// Pages opens the web page of a webview or camera view. Nil, or a system without a web view, makes those views
	// say the system cannot show them.
	Pages platform.WebPages
	// Permissions reads the OS camera permission before a camera view starts, and asks for camera and microphone
	// access for a webview that declares them. Nil skips both.
	Permissions platform.Permissions
	// Cameras remembers the camera each camera view chose. Nil keeps the choice for the life of the session.
	Cameras CameraMemory
	// IsDark reports the palette's appearance, which a web page's theme follows. Nil means light.
	IsDark func() bool
}

// Session is one open extension view. It is used on the UI thread only.
type Session struct {
	deps         Deps
	extID        string
	view         api.View
	host         search.Host
	items        []api.Item
	sections     []api.ItemSection
	loading      bool
	failure      *api.Failure
	hasMore      bool
	selectID     string
	images       map[string]*ui.Bitmap
	requested    map[string]bool
	handles      map[handle]route
	markdown     *markdown.Options
	started      bool
	immediate    bool
	dismissed    bool
	refreshing   bool
	stopInterval chan struct{}
	form         *formState
	media        *palette.View
	page         platform.WebPage
	camera       *cameraView
	searchChange searchChange
}

// NewSession prepares view for the extension extID. Nothing runs until Open shows it.
func NewSession(deps Deps, extID string, view api.View) *Session {
	if deps.Cameras == nil {
		deps.Cameras = NewMemoryCameras()
	}
	s := &Session{
		deps: deps, extID: extID, view: view,
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
	}
	return s.unsupportedPage()
}

// listPage is the searchable list of items, with its search accessory, its loading state and its close hook.
func (s *Session) listPage() *palette.View {
	page := &palette.View{Title: s.view.Title, Placeholder: s.view.Placeholder, Presentation: palettePresentation(s.view.Presentation)}
	page.Search = func(q string) []search.Item {
		page.SelectID = s.takeSelectID()
		s.scheduleSearchChange(q)
		return s.rows(q)
	}
	page.Loading = s.isLoadingEmpty
	page.Pending = s.searchChangePending
	page.EmptyTitle, page.EmptySub = s.emptyTitle(), s.emptySubtitle()
	page.OnLeave = s.close
	page.Accessory = s.searchAccessory()
	return page
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
	page.Grid = &palette.Grid{Columns: s.gridColumns(), CellHeight: gridCellHeight, Cell: drawGridCell}
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

// isLoadingEmpty is true while a data source has nothing to show yet; the palette then shows no empty state.
func (s *Session) isLoadingEmpty() bool {
	return s.loading && len(s.items) == 0 && len(s.sections) == 0
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
