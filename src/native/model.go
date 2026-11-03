package main

import (
	"github.com/egoist/mygo/ui"
)

// Action is something you can do with a result: the first one runs on Enter, all are in Ctrl+K.
type Action struct {
	Title string
	Mods  ui.Modifiers
	Key   ui.Key
	Label string // shortcut as displayed, e.g. "⌃⌫"
	Run   func(p *Palette)
}

// Item is one row in the palette.
type Item struct {
	Kind      string // label shown at the right edge: Application, Command, ...
	Title     string
	Subtitle  string
	Glyph     string // 1-2 char avatar when there is no icon
	Icon      string // lucide icon name
	Section   string // group header shown above the first row of a run
	Tone      string // yellow|blue|purple|green|red|orange|pink
	Prominent bool   // accent-tinted row (Automate)
	TintTitle bool   // title in the tone's color
	IconPath  string // file whose system icon is shown
	Image     *ui.Bitmap
	Accessory string
	Actions   []Action
	Detail    func(c *ui.Context, p *Palette) // preview pane
	ID        string
}

// View is one level of the palette navigation stack.
type View struct {
	Title       string
	TitleFn     func() string
	Placeholder string
	Search      func(q string) []Item
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
