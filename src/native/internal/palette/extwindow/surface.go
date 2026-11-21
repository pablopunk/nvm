package extwindow

import (
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Spec is what a window opens with: its title, its size, its place when it remembers one, and the chrome and level the
// options ask for.
type Spec struct {
	Title              string
	Width, Height      int
	Saved              *Frame
	Position           string
	AlwaysOnTop        bool
	VisibleOnAllSpaces bool
	TitleBarHidden     bool
	Frameless          bool
	// NoShadow drops the window's shadow, and with it the client-side border and shadow GTK draws on Wayland.
	NoShadow bool
}

// Surface is one open window. It is used on the UI thread only. Show and Hide keep the palette page's state, as a
// palette does; Close ends the window and runs the OnClosed callbacks.
type Surface interface {
	Show()
	Hide()
	Close()
	IsVisible() bool
	// SetRoot replaces the page the window shows, without closing the window.
	SetRoot(root *palette.View)
	// Host is the search host of the window's palette: the host its views' loads and refreshes answer to.
	Host() search.Host
	// Reconfigure applies the spec of an open window: its title, size, level and spaces.
	Reconfigure(spec Spec)
	Frame() Frame
	OnFrameChanged(fn func())
	OnBlur(fn func())
	OnClosed(fn func())
}

// Opener makes the native window of a spec. The window starts hidden and has no page; the manager gives it one.
type Opener interface {
	Open(spec Spec) Surface
}
