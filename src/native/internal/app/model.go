package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// View is one level of the palette navigation stack.
type View struct {
	Title       string
	TitleFn     func() string
	Placeholder string
	Search      func(q string) []search.Item
	// Body replaces the result list (chat, editor, emoji grid, forms).
	Body func(c *ui.Context, p *Palette)
	// OnSubmit receives the input text when Enter is pressed in a Body view.
	OnSubmit             func(q string)
	HideInput            bool
	EmptyTitle, EmptySub string
	KeepQuery            bool          // do not clear the input on Enter
	Status               func() string // left footer text
	Hint                 string        // right footer text
	OnLeave              func()
	// Arrow keys are given to the body view instead of moving the list.
	OnKey func(c *ui.Context, p *Palette) bool
}
