package api

import (
	"errors"
	"fmt"
)

// ViewKind selects the host-rendered surface. Every kind except list and grid also carries one
// kind-specific spec; Validate enforces that exactly the matching spec is present.
type ViewKind string

const (
	ViewList        ViewKind = "list"
	ViewGrid        ViewKind = "grid"
	ViewCollection  ViewKind = "collection"
	ViewPreview     ViewKind = "preview"
	ViewFilePreview ViewKind = "file-preview"
	ViewProgress    ViewKind = "progress"
	ViewForm        ViewKind = "form"
	ViewEditor      ViewKind = "editor"
	ViewCamera      ViewKind = "camera"
	ViewWebview     ViewKind = "webview"
	ViewChat        ViewKind = "chat"
)

// ViewSize is the named dimension preset for independent windows and the palette.
type ViewSize string

const (
	SizeDefault ViewSize = "default"
	SizeLarge   ViewSize = "large"
)

// Presentation controls palette framing. A view-producing command invoked while another view is
// open is presented below it; dismissing the command restores the parent without reopening it.
type Presentation string

const (
	PresentRoot        Presentation = "root"
	PresentStacked     Presentation = "stacked"
	PresentPreview     Presentation = "preview"
	PresentSidePreview Presentation = "side-preview"
)

// View is the wire form of a host-rendered view. Authors build it with the author package helpers
// (api.List, api.Grid, ...), which set Kind and the matching spec for them.
type View struct {
	ID    string   `json:"id,omitempty"`
	Kind  ViewKind `json:"kind"`
	Title string   `json:"title"`
	// Subtitle shows above grid tiles and in editor headers. Lists and forms do not show it, so put
	// status a user must see (counts, last run) into an item or field.
	Subtitle     string       `json:"subtitle,omitempty"`
	Size         ViewSize     `json:"size,omitempty"`
	Presentation Presentation `json:"presentation,omitempty"`
	// ContentSizing is "fill" (default) or "fit" for short status-style lists.
	ContentSizing string `json:"contentSizing,omitempty"`
	// Placeholder is the search input placeholder.
	Placeholder string `json:"placeholder,omitempty"`
	// Status replaces the footer's title, such as counts. Refresh it with ViewPatch.Status.
	Status string `json:"status,omitempty"`
	// WindowCompact renders the result as a compact overlay in independent windows.
	WindowCompact bool `json:"windowCompact,omitempty"`

	// Items and Sections feed list and grid views. Source replaces Items with a lazily loaded
	// set; the host owns the loading lifecycle and EmptyView is then required.
	Items    []Item        `json:"items,omitempty"`
	Sections []ItemSection `json:"sections,omitempty"`
	Source   *DataSource   `json:"source,omitempty"`

	IsLoading         bool             `json:"isLoading,omitempty"`
	EmptyView         *EmptyState      `json:"emptyView,omitempty"`
	Detail            *DetailPane      `json:"detail,omitempty"`
	Pagination        *Pagination      `json:"pagination,omitempty"`
	SearchAccessory   *SearchAccessory `json:"searchAccessory,omitempty"`
	Refresh           *Refresh         `json:"refresh,omitempty"`
	SelectedItemID    string           `json:"selectedItemId,omitempty"`
	MaxVisibleItems   int              `json:"maxVisibleItems,omitempty"`
	OnSelectionChange *ActionRef       `json:"onSelectionChange,omitempty"`
	// OnSearchChange runs for the trimmed search text after the user pauses typing. Its patch may replace the items, and
	// its SearchResultIds are a second batch of those ids, shown after the local matches. Only list and grid views carry it.
	OnSearchChange *ActionRef `json:"onSearchChange,omitempty"`
	// Search answers every typed query with the rows of the search, in the order the handler gives them. The host does not
	// filter or sort them, and shows the rows of the latest answer while the next one runs. The handler gets the trimmed
	// text as Invocation.Value, and the value of SearchAccessory as Ref.Args under the accessory's id. It runs off the
	// UI thread under the search budget, so it must answer quickly. Only list and grid views carry it.
	Search *ActionRef `json:"search,omitempty"`
	// Hint is the right footer text of a list or grid view, such as the keys it answers. Empty shows the host's default.
	Hint string `json:"hint,omitempty"`

	Actions               []Action        `json:"actions,omitempty"`
	ActionPanel           *ActionPanel    `json:"actionPanel,omitempty"`
	ActionPanelVisibility PanelVisibility `json:"actionPanelVisibility,omitempty"`

	// Kind-specific specs. Exactly the one matching Kind may be set.
	Grid       *GridSpec       `json:"grid,omitempty"`
	Collection *CollectionSpec `json:"collection,omitempty"`
	Preview    *PreviewSpec    `json:"preview,omitempty"`
	File       *FilePreview    `json:"file,omitempty"`
	Editor     *EditorSpec     `json:"editor,omitempty"`
	Form       *FormSpec       `json:"form,omitempty"`
	Progress   *ProgressSpec   `json:"progress,omitempty"`
	Camera     *CameraSpec     `json:"camera,omitempty"`
	Webview    *WebviewSpec    `json:"webview,omitempty"`
	Chat       *ChatSpec       `json:"chat,omitempty"`
}

// specKinds maps each spec field to the single kind it belongs to.
func (v View) specKinds() map[ViewKind]bool {
	return map[ViewKind]bool{
		ViewGrid:        v.Grid != nil,
		ViewCollection:  v.Collection != nil,
		ViewPreview:     v.Preview != nil,
		ViewFilePreview: v.File != nil,
		ViewEditor:      v.Editor != nil,
		ViewForm:        v.Form != nil,
		ViewProgress:    v.Progress != nil,
		ViewCamera:      v.Camera != nil,
		ViewWebview:     v.Webview != nil,
		ViewChat:        v.Chat != nil,
	}
}

// Validate checks that the view kind is known, that only its own spec is set, and that every
// nested action and item is valid. Item ids must be unique within the view.
func (v View) Validate() error {
	if !oneOf(v.Kind, ViewList, ViewGrid, ViewCollection, ViewPreview, ViewFilePreview, ViewProgress,
		ViewForm, ViewEditor, ViewCamera, ViewWebview, ViewChat) {
		return fmt.Errorf("unknown view kind %q", v.Kind)
	}
	if v.Title == "" {
		return errors.New("view title is required")
	}
	if v.Search != nil && v.Kind != ViewList && v.Kind != ViewGrid {
		return fmt.Errorf("view %q: search needs a list or grid view, not %q", v.Title, v.Kind)
	}
	if v.OnSearchChange != nil && v.Kind != ViewList && v.Kind != ViewGrid {
		return fmt.Errorf("view %q: onSearchChange needs a list or grid view, not %q", v.Title, v.Kind)
	}
	for kind, present := range v.specKinds() {
		if present != (kind == v.Kind) {
			return fmt.Errorf("view kind %q must set exactly its own spec (found spec for %q)", v.Kind, kind)
		}
	}
	if v.Source != nil && v.EmptyView == nil {
		return errors.New("a view with a data source needs emptyView")
	}
	if v.Collection != nil {
		if err := v.Collection.Validate(); err != nil {
			return err
		}
	}
	if v.Form != nil {
		if err := v.Form.Validate(); err != nil {
			return err
		}
	}
	for _, action := range v.Actions {
		if err := action.Validate(); err != nil {
			return fmt.Errorf("view %q: %w", v.Title, err)
		}
	}
	return validateItems(v.Items)
}

func validateItems(items []Item) error {
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if item.ID == "" {
			return errors.New("every item needs an id")
		}
		if seen[item.ID] {
			return fmt.Errorf("duplicate item id %q", item.ID)
		}
		seen[item.ID] = true
		if err := item.validateDragImage(); err != nil {
			return fmt.Errorf("item %q: %w", item.ID, err)
		}
	}
	return nil
}
