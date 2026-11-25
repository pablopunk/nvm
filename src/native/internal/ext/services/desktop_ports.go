package services

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
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
	Apply(ctx context.Context, layout api.WindowLayout, options api.WindowManagementOptions) (api.ApplyResult, error)
	FocusNeighbor(ctx context.Context, direction string, wrap bool) (api.FocusResult, error)
}

// AccountState reports whether the palette is signed in to a Nevermind account.
type AccountState interface {
	SignedIn() bool
}

// ShortcutTable reads the shortcuts the user has set up. The shortcuts manager implements it.
type ShortcutTable interface {
	List() []api.ShortcutRecord
	Palette() api.ShortcutRecord
	Aliases(prefix string) map[string][]string
}

// ShortcutEditor changes the user's global shortcuts. Bindable and Plan read a snapshot and never wait. Set and Remove
// run on the UI thread and wait for it; when ctx ends before the change starts, nothing is saved. A refused change, or
// one whose expect revision is stale, fails with api.ErrInvalidInput and saves nothing.
type ShortcutEditor interface {
	Bindable() []api.ShortcutRecord
	// Plan prepares setting accelerator for actionID, or removing its shortcut when accelerator is empty.
	Plan(actionID, accelerator string) (protocol.ShortcutChange, error)
	Set(ctx context.Context, actionID, accelerator, expect string) (api.ShortcutRecord, error)
	Remove(ctx context.Context, actionID, expect string) error
}

// AliasEditor changes the user's aliases: the phrases that find an action in the root search. An alias belongs to one
// action at a time. A refused change fails with api.ErrInvalidInput and saves nothing.
type AliasEditor interface {
	// PlanAlias prepares adding alias to actionID, or removing alias when actionID is empty, without saving anything.
	PlanAlias(actionID, alias string) (protocol.AliasChange, error)
	AddAlias(actionID, alias string) error
	RemoveAlias(alias string) error
}
