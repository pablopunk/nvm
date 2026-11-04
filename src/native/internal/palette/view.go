package palette

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// Presentation says how a view is shown. Stacked is the zero value: a view pushed above the root. Root is the
// palette's first level. Preview and SidePreview are views shown beside the list; the shell carries the
// setting, and drawing them is the work of the extension views (PLAN.md WP3.3).
type Presentation int

const (
	Stacked Presentation = iota
	Root
	Preview
	SidePreview
)

// View is one level of the navigation stack. Either Search (answered on the UI thread, for views that read live
// state) or Providers (answered off the UI thread, through the view's coordinator) supplies the rows; a View
// with Body draws its own content in place of the list.
type View struct {
	Title        string
	TitleFn      func() string
	Placeholder  string
	Presentation Presentation

	Search    func(q string) []search.Item
	Providers func() []search.Provider
	coord     *search.Coordinator

	// Body replaces the result list (chat, editor, emoji grid, forms).
	Body func(c *ui.Context, h search.Host)
	// Grid lays the rows out as cells instead of a list. Rows keep their actions, the action menu and the footer.
	Grid *Grid
	// Limit is how many rows the view keeps from its providers. Zero means search.DefaultLimit.
	Limit int
	// OnSubmit receives the input text when Enter is pressed in a Body view.
	OnSubmit  func(q string)
	HideInput bool
	// Concealed masks the search field's text while the view is open, for a secret such as a password. The query is
	// never remembered or learned while it is set.
	Concealed            bool
	EmptyTitle, EmptySub string
	KeepQuery            bool          // do not clear the input on Enter
	Status               func() string // not drawn; kept for the views that set it
	// Footer is the left footer text when the view follows its chosen row, such as the picker's character name.
	// It receives the chosen row, or nil without one, and replaces the title.
	Footer func(chosen *search.Item) string
	Hint   string // right footer text
	// HintFn, when set, gives the right footer text instead of Hint; it is read on every frame.
	HintFn  func() string
	OnLeave func()
	// OnHide runs on every page of the stack when the palette hides. The pages stay on the stack until the next Show,
	// so a question waiting for an answer can end at once.
	OnHide func()
	// Loading reports that the view has no rows yet because they are still arriving; no empty state shows meanwhile.
	Loading func() bool
	// Accessory is a small choice at the end of the search field, such as a category. Its change is passed to OnChange.
	Accessory *Accessory
	// SelectID names the row to choose once the view's rows first show, such as an item the extension selected.
	SelectID string
	// OnKey sees the keys before the list and the action menu do; it returns true when it used one.
	OnKey func(c *ui.Context, h search.Host) bool
	// OnEscape sees Escape before the palette clears the query, leaves the page or hides. It returns true when the page
	// used the key, such as to stop a reply in flight.
	OnEscape func() bool
	// SurvivesHide keeps the page open across a hide, as Electron keeps the last live AI chat: the next summon shows it
	// and the pages pushed above it are dropped. Every other page is left when the palette hides.
	SurvivesHide bool
	// Input, when set, is the handle of a text input the Body draws itself, as a chat's composer does. The palette then
	// draws no search row, focuses this input instead of its own, and the Body keeps the text.
	Input *ui.Handle
	// OnActive reports whether the view is the page on top of a shown palette. It is called when the view is pushed
	// or covered, when the palette shows or hides, and when the view is the top page at either. A covered or hidden
	// view must stop what only a visible page may do, such as a camera.
	OnActive func(active bool)
}

// Grid lays a view's rows out as cells, Columns to a row, as the emoji picker does. The cells share the pane's
// width equally, so the grid fills the pane. The palette moves the choice in two dimensions with the arrows, Home,
// End, Page Up and Page Down, draws each cell's plate, selection and clicks, and labels the cell with the row's
// title. Cell draws what a cell shows and nothing else.
type Grid struct {
	Columns    int
	CellHeight float32
	Cell       func(c *ui.Context, it search.Item)
}

// Accessory is a search field's option picker: each click moves to the next option and reports its value.
type Accessory struct {
	Value    string
	Options  []AccessoryOption
	OnChange func(value string)
}

// AccessoryOption is one choice of an Accessory.
type AccessoryOption struct {
	Title string
	Value string
}

// Label is the accessory's current option title, or "" when no option matches its value.
func (a *Accessory) Label() string {
	for _, option := range a.Options {
		if option.Value == a.Value {
			return option.Title
		}
	}
	return ""
}

// Next moves to the option after the current one, wrapping at the end, and reports the new value.
func (a *Accessory) Next() string {
	if len(a.Options) == 0 {
		return a.Value
	}
	index := 0
	for i, option := range a.Options {
		if option.Value == a.Value {
			index = (i + 1) % len(a.Options)
			break
		}
	}
	a.Value = a.Options[index].Value
	return a.Value
}
