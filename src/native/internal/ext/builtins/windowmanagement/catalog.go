package windowmanagement

import "github.com/pablopunk/nvm/native/internal/ext/api"

// layoutIDPrefix leads the command IDs of the layouts and the focus moves. Their shortcut keys are the 1.x action keys,
// which keep the prefix "layout." too, so a shortcut the user set in 1.x still binds.
const layoutIDPrefix = "layout."

// actionKeyPrefix leads the shortcut keys of this built-in's commands. It is the 1.x prefix for actions of the
// window-management extension.
const actionKeyPrefix = "extension-action:window-management:"

// presentation is how a command reads in the palette: its title, its icon and the words that match it. The strings are
// the Electron app's.
type presentation struct {
	title    string
	icon     string
	keywords []string
}

func present(title, icon string, keywords ...string) presentation {
	return presentation{title: title, icon: icon, keywords: keywords}
}

// layoutSpec is a layout's wire layout. Layouts that the settings size take the settings they read.
type layoutSpec func(Settings) api.WindowLayout

// layout is one layout command: its ID, its presentation and what it applies.
type layout struct {
	id           string
	presentation presentation
	spec         layoutSpec
}

func fixed(spec api.WindowLayout) layoutSpec {
	return func(Settings) api.WindowLayout { return spec }
}

func fraction(x, y, width, height float64, cycleGroup string) api.WindowLayout {
	return api.WindowLayout{Kind: api.LayoutFraction, Fraction: &api.Rect{X: x, Y: y, Width: width, Height: height}, CycleGroup: cycleGroup}
}

const (
	half     = 1.0 / 2
	third    = 1.0 / 3
	twoThird = 2.0 / 3
	sixth    = 1.0 / 6
	fourth   = 1.0 / 4
)

// layouts is the catalog of layout commands, in the order the palette and the shortcut list show them.
var layouts = []layout{
	{"left-half", present("Left Half", "panel-left", "left", "half"), fixed(fraction(0, 0, half, 1, "left"))},
	{"right-half", present("Right Half", "panel-right", "right", "half"), fixed(fraction(half, 0, half, 1, "right"))},
	{"top-half", present("Top Half", "panel-top", "top", "half"), fixed(fraction(0, 0, 1, half, "top"))},
	{"bottom-half", present("Bottom Half", "panel-bottom", "bottom", "half"), fixed(fraction(0, half, 1, half, "bottom"))},
	{"center-half", present("Center Half", "square-dashed", "center", "middle", "half"), func(s Settings) api.WindowLayout {
		width := s.CenterHalfWidthPercent / percent
		return fraction((1-width)/2, 0, width, 1, "")
	}},
	{"maximize", present("Maximize", "maximize", "max", "full", "fill"), fixed(fraction(0, 0, 1, 1, ""))},
	{"center", present("Center", "scan", "middle", "centre"), fixed(api.WindowLayout{Kind: api.LayoutCenter})},
	{"first-third", present("First Third", "columns-3", "left", "third", "column"), fixed(fraction(0, 0, third, 1, ""))},
	{"center-third", present("Center Third", "columns-3", "middle", "third", "column"), fixed(fraction(third, 0, third, 1, ""))},
	{"last-third", present("Last Third", "columns-3", "right", "third", "column"), fixed(fraction(twoThird, 0, third, 1, ""))},
	{"top-third", present("Top Third", "rows-3", "top", "third", "row"), fixed(fraction(0, 0, 1, third, ""))},
	{"middle-third", present("Middle Third", "rows-3", "middle", "third", "row"), fixed(fraction(0, third, 1, third, ""))},
	{"bottom-third", present("Bottom Third", "rows-3", "bottom", "third", "row"), fixed(fraction(0, twoThird, 1, third, ""))},
	{"first-two-thirds", present("First Two Thirds", "columns-2", "left", "two-thirds", "column"), fixed(fraction(0, 0, twoThird, 1, ""))},
	{"center-two-thirds", present("Center Two Thirds", "columns-2", "middle", "two-thirds", "column"), fixed(fraction(sixth, 0, twoThird, 1, ""))},
	{"last-two-thirds", present("Last Two Thirds", "columns-2", "right", "two-thirds", "column"), fixed(fraction(third, 0, twoThird, 1, ""))},
	{"top-two-thirds", present("Top Two Thirds", "rows-2", "top", "two-thirds", "row"), fixed(fraction(0, 0, 1, twoThird, ""))},
	{"middle-two-thirds", present("Middle Two Thirds", "rows-2", "middle", "two-thirds", "row"), fixed(fraction(0, sixth, 1, twoThird, ""))},
	{"bottom-two-thirds", present("Bottom Two Thirds", "rows-2", "bottom", "two-thirds", "row"), fixed(fraction(0, third, 1, twoThird, ""))},
	{"top-left-quarter", present("Top Left Quarter", "grid-2x2", "top", "left", "quarter"), fixed(fraction(0, 0, half, half, ""))},
	{"top-right-quarter", present("Top Right Quarter", "grid-2x2", "top", "right", "quarter"), fixed(fraction(half, 0, half, half, ""))},
	{"bottom-left-quarter", present("Bottom Left Quarter", "grid-2x2", "bottom", "left", "quarter"), fixed(fraction(0, half, half, half, ""))},
	{"bottom-right-quarter", present("Bottom Right Quarter", "grid-2x2", "bottom", "right", "quarter"), fixed(fraction(half, half, half, half, ""))},
	{"first-fourth", present("First Fourth", "columns-4", "first", "fourth", "column"), fixed(fraction(0, 0, fourth, 1, ""))},
	{"second-fourth", present("Second Fourth", "columns-4", "second", "fourth", "column"), fixed(fraction(fourth, 0, fourth, 1, ""))},
	{"third-fourth", present("Third Fourth", "columns-4", "third", "fourth", "column"), fixed(fraction(half, 0, fourth, 1, ""))},
	{"last-fourth", present("Last Fourth", "columns-4", "last", "fourth", "column"), fixed(fraction(half+fourth, 0, fourth, 1, ""))},
	{"top-left-sixth", present("Top Left Sixth", "grid-3x3", "top", "left", "sixth"), fixed(fraction(0, 0, third, half, ""))},
	{"top-center-sixth", present("Top Center Sixth", "grid-3x3", "top", "middle", "sixth"), fixed(fraction(third, 0, third, half, ""))},
	{"top-right-sixth", present("Top Right Sixth", "grid-3x3", "top", "right", "sixth"), fixed(fraction(twoThird, 0, third, half, ""))},
	{"bottom-left-sixth", present("Bottom Left Sixth", "grid-3x3", "bottom", "left", "sixth"), fixed(fraction(0, half, third, half, ""))},
	{"bottom-center-sixth", present("Bottom Center Sixth", "grid-3x3", "bottom", "middle", "sixth"), fixed(fraction(third, half, third, half, ""))},
	{"bottom-right-sixth", present("Bottom Right Sixth", "grid-3x3", "bottom", "right", "sixth"), fixed(fraction(twoThird, half, third, half, ""))},
	{"maximize-width", present("Maximize Width", "move-horizontal", "width", "full", "horizontal"), fixed(api.WindowLayout{Kind: api.LayoutMaximizeWidth})},
	{"maximize-height", present("Maximize Height", "move-vertical", "height", "full", "vertical"), fixed(api.WindowLayout{Kind: api.LayoutMaximizeHeight})},
	{"almost-maximize", present("Almost Maximize", "expand", "almost", "max", "margin"), func(s Settings) api.WindowLayout {
		return api.WindowLayout{Kind: api.LayoutScaledCenter, WidthFraction: s.AlmostMaximizeWidthPercent / percent, HeightFraction: s.AlmostMaximizeHeightPercent / percent}
	}},
	{"reasonable-size", present("Reasonable Size", "app-window", "reasonable", "size", "comfortable"), func(s Settings) api.WindowLayout {
		return api.WindowLayout{Kind: api.LayoutScaledCenter, WidthFraction: s.ReasonableSizeWidthPercent / percent, HeightFraction: s.ReasonableSizeHeightPercent / percent}
	}},
	{"move-left", present("Move Left", "move-left", "left", "edge", "move"), fixed(api.WindowLayout{Kind: api.LayoutEdge, Edge: "left"})},
	{"move-right", present("Move Right", "move-right", "right", "edge", "move"), fixed(api.WindowLayout{Kind: api.LayoutEdge, Edge: "right"})},
	{"move-up", present("Move Up", "move-up", "up", "top", "edge", "move"), fixed(api.WindowLayout{Kind: api.LayoutEdge, Edge: "top"})},
	{"move-down", present("Move Down", "move-down", "down", "bottom", "edge", "move"), fixed(api.WindowLayout{Kind: api.LayoutEdge, Edge: "bottom"})},
	{"make-larger", present("Make Larger", "zoom-in", "larger", "bigger", "grow", "expand"), resizeStep("larger")},
	{"make-smaller", present("Make Smaller", "zoom-out", "smaller", "shrink", "reduce"), resizeStep("smaller")},
	{"next-display", present("Next Display", "monitor", "display", "monitor", "screen", "next"), fixed(api.WindowLayout{Kind: api.LayoutDisplay, Direction: "next"})},
	{"previous-display", present("Previous Display", "monitor", "display", "monitor", "screen", "previous"), fixed(api.WindowLayout{Kind: api.LayoutDisplay, Direction: "previous"})},
	{"restore", present("Restore", "undo-2", "restore", "original", "back", "undo"), fixed(api.WindowLayout{Kind: api.LayoutRestore})},
	{"toggle-full-screen", present("Toggle Full Screen", "fullscreen", "fullscreen", "full", "screen", "toggle"), fixed(api.WindowLayout{Kind: api.LayoutToggleFullScreen})},
}

// percent is the scale of the percentage settings.
const percent = 100

// resizeStep grows or shrinks by the resize step setting, as a fraction of the work area.
func resizeStep(direction string) layoutSpec {
	return func(s Settings) api.WindowLayout {
		return api.WindowLayout{Kind: api.LayoutResizeStep, Direction: direction, Step: s.ResizeStepPercent / percent}
	}
}

// focusDirections are the focus moves, in the order the palette lists them.
var focusDirections = []struct {
	direction string
	presentation
}{
	{"left", present("Focus Window Left", "arrow-left", "focus", "select", "left")},
	{"right", present("Focus Window Right", "arrow-right", "focus", "select", "right")},
	{"up", present("Focus Window Up", "arrow-up", "focus", "select", "above", "up")},
	{"down", present("Focus Window Down", "arrow-down", "focus", "select", "below", "down")},
}

// desktopMove is a Next or Previous Desktop command: its ID, the direction it moves and its presentation.
type desktopMove struct {
	id, direction string
	presentation
}

// desktopMoves are the Next and Previous Desktop commands.
var desktopMoves = []desktopMove{
	{"next-desktop", "next", present("Next Desktop", "arrow-right", "space", "workspace", "mission control", "right")},
	{"previous-desktop", "previous", present("Previous Desktop", "arrow-left", "space", "workspace", "mission control", "left")},
}

// actionKey is the 1.x shortcut key of a command of this built-in.
func actionKey(id string) string {
	return actionKeyPrefix + id
}

// layoutCommandID is the command ID of a layout or focus move, which the 1.x keys name as "layout.<id>".
func layoutCommandID(id string) string {
	return layoutIDPrefix + id
}
