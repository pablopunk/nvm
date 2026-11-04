// Package commands is the catalog of window management commands (WP4.1c): every layout and focus action the
// palette lists, with the stable IDs that shortcut bindings refer to, and the runner that applies a command
// through the engine with the user's settings. It has no palette or OS dependency.
package commands

import "github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"

// idPrefix leads every command ID. IDs are persisted by shortcut bindings, so they never change.
const idPrefix = "window."

// Command is one window action. A layout applies a geometry preset to the target window; a focus command raises
// the nearest window in a direction from it.
type Command struct {
	ID       string
	Title    string
	Icon     string
	Keywords []string

	preset    geometry.PresetID
	direction geometry.NeighborDirection
}

// IsFocus reports whether the command raises a neighbor window instead of placing the target.
func (c Command) IsFocus() bool {
	return c.direction != ""
}

// Preset returns the layout preset a layout command applies. It is empty for focus commands.
func (c Command) Preset() geometry.PresetID {
	return c.preset
}

// Direction returns the direction a focus command moves in. It is empty for layout commands.
func (c Command) Direction() geometry.NeighborDirection {
	return c.direction
}

// All lists every window command: the layouts in catalog order, then the four focus commands.
func All() []Command {
	commands := make([]Command, 0, len(geometry.PresetIDs())+len(focusDirections))
	for _, id := range geometry.PresetIDs() {
		commands = append(commands, layoutCommand(id))
	}
	for _, direction := range focusDirections {
		commands = append(commands, focusCommand(direction))
	}
	return commands
}

// ByID returns the command with the given ID, if the catalog has one.
func ByID(id string) (Command, bool) {
	command, ok := commandsByID[id]
	return command, ok
}

var focusDirections = []geometry.NeighborDirection{
	geometry.NeighborLeft, geometry.NeighborRight, geometry.NeighborUp, geometry.NeighborDown,
}

var commandsByID = indexByID(All())

func indexByID(commands []Command) map[string]Command {
	index := make(map[string]Command, len(commands))
	for _, command := range commands {
		index[command.ID] = command
	}
	return index
}

func layoutCommand(id geometry.PresetID) Command {
	presentation := layoutPresentations[id]
	return Command{
		ID:       idPrefix + string(id),
		Title:    presentation.title,
		Icon:     presentation.icon,
		Keywords: presentation.keywords,
		preset:   id,
	}
}

func focusCommand(direction geometry.NeighborDirection) Command {
	presentation := focusPresentations[direction]
	return Command{
		ID:        idPrefix + "focus-" + string(direction),
		Title:     presentation.title,
		Icon:      presentation.icon,
		Keywords:  presentation.keywords,
		direction: direction,
	}
}
