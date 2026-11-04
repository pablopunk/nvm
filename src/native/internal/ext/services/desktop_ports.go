package services

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// FrontmostSource names the app that has the keyboard. The platform window port implements it; nil where the OS has
// none.
type FrontmostSource interface {
	Frontmost(ctx context.Context) (platform.WindowApp, bool, error)
}

// WindowEngine places and lists windows of other apps. The window management engine implements it; this layer sits
// below that engine, so it cannot name the engine's type. Runtime failures come back as results with OK false.
type WindowEngine interface {
	Displays(ctx context.Context) ([]api.DisplayInfo, error)
	Windows(ctx context.Context, query api.WindowQuery) ([]api.WindowInfo, error)
	Focus(ctx context.Context, handle api.WindowHandle) (api.FocusResult, error)
	ApplyTo(ctx context.Context, handle api.WindowHandle, layout api.WindowLayout, options api.WindowManagementOptions) (api.ApplyResult, error)
}

// ShortcutTable reads the shortcuts the user has set up. The shortcuts manager implements it.
type ShortcutTable interface {
	List() []api.ShortcutRecord
	Palette() api.ShortcutRecord
}
