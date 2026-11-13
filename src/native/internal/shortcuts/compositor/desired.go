package compositor

import (
	"maps"
	"slices"

	"github.com/pablopunk/nvm/native/internal/platform/hyprland"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/shortcuts/actioncli"
)

// Identity is how the compositor runs and recognizes the app: the executable a binding runs, and the app IDs of its
// windows.
type Identity struct {
	Executable string
	Classes    []string
}

// DesiredFor is the module for the user's shortcuts: the palette key toggles the palette, and every other bound action
// runs through the action command line, so a key also starts Nevermind when it is not running. The app's windows (matched
// by their app ID only) carry no compositor border.
func DesiredFor(palette string, actions map[string]string, id Identity) hyprland.Desired {
	var d hyprland.Desired
	if palette != "" {
		d.Binds = append(d.Binds, bindFor(palette, shortcuts.PaletteToggleID, id.Executable))
	}
	for _, actionID := range slices.Sorted(maps.Keys(actions)) {
		if accelerator := actions[actionID]; accelerator != "" && actionID != shortcuts.PaletteID {
			d.Binds = append(d.Binds, bindFor(accelerator, actionID, id.Executable))
		}
	}
	d.Appearance = hyprland.Appearance{Undecorated: true, Classes: id.Classes}
	return d
}

func bindFor(accelerator, actionID, executable string) hyprland.Bind {
	return hyprland.Bind{
		Accelerator: shortcuts.Normalize(accelerator),
		// exec runs the command through sh -c in both config formats.
		Command: actioncli.Invocation(executable, actionID, "linux"),
	}
}
