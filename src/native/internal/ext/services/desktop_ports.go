package services

import (
	"context"
	"encoding/json"

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

// ExtensionsEditor lists the installed extensions and changes their state for the extensions service. Nothing on it
// enables an extension: OpenReview shows the capability review of a disabled extension in the palette and returns at
// once, and only the user enables it there. A refusal, such as a built-in or an unknown id, wraps api.ErrInvalidInput
// and changes nothing.
type ExtensionsEditor interface {
	List() []api.ExtensionRecord
	Disable(id string) error
	OpenReview(id string) error
	SetMenuBarItem(id string, shown bool) error
}

// PreferenceEditor reads and changes the declared settings of every installed extension for the user, as their
// settings pages do; secrets are never among them. Each call runs on the UI thread and waits for it; when ctx ends
// before a call starts, it does nothing. A refused change, or one whose expect revision is stale, fails with
// api.ErrInvalidInput and saves nothing.
type PreferenceEditor interface {
	List(ctx context.Context, query string) ([]api.PreferenceRecord, error)
	// Plan prepares saving value as the setting without saving anything. It refuses an unknown setting, a value the
	// setting does not accept, a value it already holds and a setting only its own page can record.
	Plan(ctx context.Context, extensionID, settingID string, value json.RawMessage) (protocol.PreferenceChange, error)
	Set(ctx context.Context, extensionID, settingID string, value json.RawMessage, expect string) (api.PreferenceRecord, error)
}
