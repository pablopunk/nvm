package commands

import "github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"

// presentation is how a command reads in the palette: its title, its icon and the words that match it.
// The strings are the Electron app's (window-layout-presentation.ts and window-management-focus-commands.ts).
type presentation struct {
	title    string
	icon     string
	keywords []string
}

func present(title, icon string, keywords ...string) presentation {
	return presentation{title: title, icon: icon, keywords: keywords}
}

var layoutPresentations = map[geometry.PresetID]presentation{
	"left-half":            present("Left Half", "panel-left", "left", "half"),
	"right-half":           present("Right Half", "panel-right", "right", "half"),
	"top-half":             present("Top Half", "panel-top", "top", "half"),
	"bottom-half":          present("Bottom Half", "panel-bottom", "bottom", "half"),
	"center-half":          present("Center Half", "square-dashed", "center", "middle", "half"),
	"maximize":             present("Maximize", "maximize", "max", "full", "fill"),
	"center":               present("Center", "scan", "middle", "centre"),
	"first-third":          present("First Third", "columns-3", "left", "third", "column"),
	"center-third":         present("Center Third", "columns-3", "middle", "third", "column"),
	"last-third":           present("Last Third", "columns-3", "right", "third", "column"),
	"top-third":            present("Top Third", "rows-3", "top", "third", "row"),
	"middle-third":         present("Middle Third", "rows-3", "middle", "third", "row"),
	"bottom-third":         present("Bottom Third", "rows-3", "bottom", "third", "row"),
	"first-two-thirds":     present("First Two Thirds", "columns-2", "left", "two-thirds", "column"),
	"center-two-thirds":    present("Center Two Thirds", "columns-2", "middle", "two-thirds", "column"),
	"last-two-thirds":      present("Last Two Thirds", "columns-2", "right", "two-thirds", "column"),
	"top-two-thirds":       present("Top Two Thirds", "rows-2", "top", "two-thirds", "row"),
	"middle-two-thirds":    present("Middle Two Thirds", "rows-2", "middle", "two-thirds", "row"),
	"bottom-two-thirds":    present("Bottom Two Thirds", "rows-2", "bottom", "two-thirds", "row"),
	"top-left-quarter":     present("Top Left Quarter", "grid-2x2", "top", "left", "quarter"),
	"top-right-quarter":    present("Top Right Quarter", "grid-2x2", "top", "right", "quarter"),
	"bottom-left-quarter":  present("Bottom Left Quarter", "grid-2x2", "bottom", "left", "quarter"),
	"bottom-right-quarter": present("Bottom Right Quarter", "grid-2x2", "bottom", "right", "quarter"),
	"first-fourth":         present("First Fourth", "columns-4", "first", "fourth", "column"),
	"second-fourth":        present("Second Fourth", "columns-4", "second", "fourth", "column"),
	"third-fourth":         present("Third Fourth", "columns-4", "third", "fourth", "column"),
	"last-fourth":          present("Last Fourth", "columns-4", "last", "fourth", "column"),
	"top-left-sixth":       present("Top Left Sixth", "grid-3x3", "top", "left", "sixth"),
	"top-center-sixth":     present("Top Center Sixth", "grid-3x3", "top", "middle", "sixth"),
	"top-right-sixth":      present("Top Right Sixth", "grid-3x3", "top", "right", "sixth"),
	"bottom-left-sixth":    present("Bottom Left Sixth", "grid-3x3", "bottom", "left", "sixth"),
	"bottom-center-sixth":  present("Bottom Center Sixth", "grid-3x3", "bottom", "middle", "sixth"),
	"bottom-right-sixth":   present("Bottom Right Sixth", "grid-3x3", "bottom", "right", "sixth"),
	"maximize-width":       present("Maximize Width", "move-horizontal", "width", "full", "horizontal"),
	"maximize-height":      present("Maximize Height", "move-vertical", "height", "full", "vertical"),
	"almost-maximize":      present("Almost Maximize", "expand", "almost", "max", "margin"),
	"reasonable-size":      present("Reasonable Size", "app-window", "reasonable", "size", "comfortable"),
	"move-left":            present("Move Left", "move-left", "left", "edge", "move"),
	"move-right":           present("Move Right", "move-right", "right", "edge", "move"),
	"move-up":              present("Move Up", "move-up", "up", "top", "edge", "move"),
	"move-down":            present("Move Down", "move-down", "down", "bottom", "edge", "move"),
	"make-larger":          present("Make Larger", "zoom-in", "larger", "bigger", "grow", "expand"),
	"make-smaller":         present("Make Smaller", "zoom-out", "smaller", "shrink", "reduce"),
	"next-display":         present("Next Display", "monitor", "display", "monitor", "screen", "next"),
	"previous-display":     present("Previous Display", "monitor", "display", "monitor", "screen", "previous"),
	"restore":              present("Restore", "undo-2", "restore", "original", "back", "undo"),
	"toggle-full-screen":   present("Toggle Full Screen", "fullscreen", "fullscreen", "full", "screen", "toggle"),
}

var focusPresentations = map[geometry.NeighborDirection]presentation{
	geometry.NeighborLeft:  present("Focus Window Left", "arrow-left", "focus", "select", "left"),
	geometry.NeighborRight: present("Focus Window Right", "arrow-right", "focus", "select", "right"),
	geometry.NeighborUp:    present("Focus Window Up", "arrow-up", "focus", "select", "above", "up"),
	geometry.NeighborDown:  present("Focus Window Down", "arrow-down", "focus", "select", "below", "down"),
}
