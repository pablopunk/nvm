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
	// OnSubmit receives the input text when Enter is pressed in a Body view.
	OnSubmit             func(q string)
	HideInput            bool
	EmptyTitle, EmptySub string
	KeepQuery            bool          // do not clear the input on Enter
	Status               func() string // left footer text
	Hint                 string        // right footer text
	OnLeave              func()
	// OnKey sees the keys before the list and the action menu do; it returns true when it used one.
	OnKey func(c *ui.Context, h search.Host) bool
}
