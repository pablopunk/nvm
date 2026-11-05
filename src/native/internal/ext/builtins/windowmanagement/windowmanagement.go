// Package windowmanagement is the built-in that moves and resizes the windows of other apps: every layout, the focus
// moves, Next and Previous Desktop, the exact move and resize, Switch Window and the settings that shape the layouts.
// It is built on the extension API alone; the window engine and the OS sit behind ctx.Desktop().WindowManagement().
//
// Layer: ext. The package imports api, search and the standard library.
package windowmanagement

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity.
const ID = "nevermind.window-management"

// WindowManagementSubtitle is the line every window command shows under its title.
const WindowManagementSubtitle = "Window Management"

// Command IDs of the commands that are not layouts.
const (
	settingsCommandID  = "settings"
	switchCommandID    = "switch-window"
	customCommandID    = "custom-commands"
	moveWindowID       = "move-window"
	resizeWindowID     = "resize-window"
	focusWindowAction  = "focus-window"
	applyWindowAction  = "apply-window"
	exactSubmitAction  = "exact-submit"
	settingToggle      = "setting-toggle"
	settingCycle       = "setting-cycle"
	settingSet         = "setting-set"
	settingsReset      = "settings-reset"
	focusCommandPrefix = "layout.focus-"
)

// Extension is the window management built-in. start holds the values the settings file kept before the built-in kept its
// own; they are the settings' defaults until the host holds a value for each.
type Extension struct {
	start Settings
}

// New returns the built-in whose settings start from start.
func New(start Settings) Extension {
	return Extension{start: start}
}

// Manifest identifies the built-in and declares its settings. Writing them needs the settings.write capability.
func (e Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0", Identity: api.Identity{ID: ID, Title: "Window Management"},
		Capabilities: []api.Capability{api.CapabilityWindowManagement, api.CapabilitySystem, api.CapabilitySettingsWrite},
		Settings:     SettingDefinitions(e.start),
	}
}

// windowManagementSupported reports whether the system can manage other apps' windows. Where it cannot, the commands
// are omitted rather than shown disabled.
func windowManagementSupported(ctx api.Context) bool {
	return ctx.System().HasCapability("window-management")
}

// Commands lists the window commands. Layouts, focus moves and desktop moves are shortcut-able; the views are typed only.
func (Extension) Commands(ctx api.Context) ([]api.Command, error) {
	if !windowManagementSupported(ctx) {
		return nil, nil
	}
	return allCommands(), nil
}

// ShortcutCommands are the commands a keyboard shortcut can be bound to: the layouts, the focus moves and the desktop
// moves. The views are not shortcut-able. The host lists them in the Keyboard Shortcuts list.
func ShortcutCommands() []api.Command {
	var out []api.Command
	for _, command := range allCommands() {
		if command.Customizable {
			out = append(out, command)
		}
	}
	return out
}

func allCommands() []api.Command {
	commands := make([]api.Command, 0, len(layouts)+len(focusDirections)+len(desktopMoves)+6)
	for _, l := range layouts {
		commands = append(commands, inPlaceCommand(layoutCommandID(l.id), l.presentation))
	}
	for _, focus := range focusDirections {
		commands = append(commands, inPlaceCommand(layoutCommandID("focus-"+focus.direction), focus.presentation))
	}
	for _, move := range desktopMoves {
		commands = append(commands, inPlaceCommand(move.id, move.presentation))
	}
	return append(commands, viewCommands()...)
}

// inPlaceCommand is a command that runs without the palette: its row is searchable, takes a shortcut, and offers the
// settings in its action menu, as the 1.x action does. Its row is a neutral command row in the Commands section, as the
// native window rows were, so a search for "settings" ranks the Settings built-in's own row first.
func inPlaceCommand(id string, p presentation) api.Command {
	return api.Command{
		ID: id, ActionID: actionKey(id), Title: p.title, Subtitle: WindowManagementSubtitle, Icon: p.icon, Keywords: p.keywords,
		Mode: api.ModeNoView, DismissAfterRun: true, Placement: []api.Placement{api.PlacementSearch}, Customizable: true,
		Appearance: neutralAppearance, Section: search.KindCommands,
		LearnKey: "cmd:" + p.title,
		Actions:  []api.Action{api.Run(settingsCommandID, "Window Management Settings", nil)},
	}
}

// neutralAppearance is the foreground of a window command's row: none, as the 1.x action without an appearance draws it.
var neutralAppearance = &api.Appearance{Foreground: api.ColorNone}

// viewCommands are the commands that open a view: the exact forms, Switch Window, the settings and Custom Window Commands.
func viewCommands() []api.Command {
	return []api.Command{
		viewCommand(moveWindowID, "Move Window…", "move", "position exact coordinates offset place"),
		viewCommand(resizeWindowID, "Resize Window…", "scaling", "size exact dimensions width height"),
		viewCommand(switchCommandID, "Switch Window", "app-window", "window switch focus raise bring front list"),
		viewCommand(settingsCommandID, "Window Management Settings", "settings", "window management settings gap size layout"),
		viewCommand(customCommandID, "Custom Window Commands", "layout-list", "custom window commands raycast"),
	}
}

func viewCommand(id, title, icon, keywords string) api.Command {
	command := api.Command{
		ID: id, Title: title, Subtitle: WindowManagementSubtitle, Icon: icon, Keywords: strings.Fields(keywords),
		Mode: api.ModeView, Placement: []api.Placement{api.PlacementSearch},
		Appearance: neutralAppearance, Section: search.KindCommands,
		LearnKey: "cmd:" + title,
	}
	if id == moveWindowID || id == resizeWindowID {
		command.ActionID = actionKey(id)
	}
	if id == settingsCommandID {
		command.Subtitle = "Gaps, cycling, and sizes for window layouts"
	}
	return command
}

// RunAction runs the window command or view action named by Ref.ID. Views are returned as results; layouts report their
// outcome as a toast.
func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if run, ok := e.handler(in.Ref.ID); ok {
		return run(ctx, in)
	}
	return api.Result{}, fmt.Errorf("the window management built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// handler maps an action ID to what it does.
func (e Extension) handler(id string) (handlerFunc, bool) {
	if focus, ok := strings.CutPrefix(id, focusCommandPrefix); ok {
		return focusNeighborRun(focus), true
	}
	if l, ok := layoutByID(strings.TrimPrefix(id, layoutIDPrefix)); ok && strings.HasPrefix(id, layoutIDPrefix) {
		return func(ctx api.Context, _ api.Invocation) (api.Result, error) {
			return e.applyLayout(ctx, l)
		}, true
	}
	if move, ok := desktopMoveByID(id); ok {
		return func(ctx api.Context, _ api.Invocation) (api.Result, error) {
			return switchDesktop(ctx, move)
		}, true
	}
	return e.viewHandler(id)
}
