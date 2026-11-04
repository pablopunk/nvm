package geometry

// PresetID is the stable name of a built-in layout. The IDs are persisted (shortcut bindings and
// settings refer to them), so existing names must never change.
type PresetID string

const (
	// Units are fractions of the work area. Constants are untyped so each is rounded once, as in JS.
	third     = 1.0 / 3
	twoThirds = 2.0 / 3
	sixth     = 1.0 / 6
	fourth    = 1.0 / 4
	half      = 1.0 / 2
)

var presetOrder = []PresetID{
	"left-half", "right-half", "top-half", "bottom-half", "center-half", "maximize",
	"center", "first-third", "center-third", "last-third", "top-third", "middle-third",
	"bottom-third", "first-two-thirds", "center-two-thirds", "last-two-thirds",
	"top-two-thirds", "middle-two-thirds", "bottom-two-thirds", "top-left-quarter",
	"top-right-quarter", "bottom-left-quarter", "bottom-right-quarter", "first-fourth",
	"second-fourth", "third-fourth", "last-fourth", "top-left-sixth", "top-center-sixth",
	"top-right-sixth", "bottom-left-sixth", "bottom-center-sixth", "bottom-right-sixth",
	"maximize-width", "maximize-height", "almost-maximize", "reasonable-size", "move-left",
	"move-right", "move-up", "move-down", "make-larger", "make-smaller", "next-display",
	"previous-display", "restore", "toggle-full-screen",
}

var presets = map[PresetID]Layout{
	"left-half":            fractionLayout(0, 0, half, 1, EdgeLeft),
	"right-half":           fractionLayout(half, 0, half, 1, EdgeRight),
	"top-half":             fractionLayout(0, 0, 1, half, EdgeTop),
	"bottom-half":          fractionLayout(0, half, 1, half, EdgeBottom),
	"center-half":          plainFraction(fourth, 0, half, 1),
	"maximize":             plainFraction(0, 0, 1, 1),
	"center":               {Kind: KindCenter},
	"first-third":          plainFraction(0, 0, third, 1),
	"center-third":         plainFraction(third, 0, third, 1),
	"last-third":           plainFraction(twoThirds, 0, third, 1),
	"top-third":            plainFraction(0, 0, 1, third),
	"middle-third":         plainFraction(0, third, 1, third),
	"bottom-third":         plainFraction(0, twoThirds, 1, third),
	"first-two-thirds":     plainFraction(0, 0, twoThirds, 1),
	"center-two-thirds":    plainFraction(sixth, 0, twoThirds, 1),
	"last-two-thirds":      plainFraction(third, 0, twoThirds, 1),
	"top-two-thirds":       plainFraction(0, 0, 1, twoThirds),
	"middle-two-thirds":    plainFraction(0, sixth, 1, twoThirds),
	"bottom-two-thirds":    plainFraction(0, third, 1, twoThirds),
	"top-left-quarter":     plainFraction(0, 0, half, half),
	"top-right-quarter":    plainFraction(half, 0, half, half),
	"bottom-left-quarter":  plainFraction(0, half, half, half),
	"bottom-right-quarter": plainFraction(half, half, half, half),
	"first-fourth":         plainFraction(0, 0, fourth, 1),
	"second-fourth":        plainFraction(fourth, 0, fourth, 1),
	"third-fourth":         plainFraction(half, 0, fourth, 1),
	"last-fourth":          plainFraction(half+fourth, 0, fourth, 1),
	"top-left-sixth":       plainFraction(0, 0, third, half),
	"top-center-sixth":     plainFraction(third, 0, third, half),
	"top-right-sixth":      plainFraction(twoThirds, 0, third, half),
	"bottom-left-sixth":    plainFraction(0, half, third, half),
	"bottom-center-sixth":  plainFraction(third, half, third, half),
	"bottom-right-sixth":   plainFraction(twoThirds, half, third, half),
	"maximize-width":       {Kind: KindMaximizeWidth},
	"maximize-height":      {Kind: KindMaximizeHeight},
	"almost-maximize":      scaledCenter(0.9, 0.9),
	"reasonable-size":      scaledCenter(0.7, 0.7),
	"move-left":            edgeMove(EdgeLeft),
	"move-right":           edgeMove(EdgeRight),
	"move-up":              edgeMove(EdgeTop),
	"move-down":            edgeMove(EdgeBottom),
	"make-larger":          resizeStep(DirectionLarger, 0.05),
	"make-smaller":         resizeStep(DirectionSmaller, 0.05),
	"next-display":         displayMove(DirectionNext),
	"previous-display":     displayMove(DirectionPrevious),
	"restore":              {Kind: KindRestore},
	"toggle-full-screen":   {Kind: KindToggleFullScreen},
}

// PresetIDs returns every built-in layout ID in catalog order.
func PresetIDs() []PresetID {
	return append([]PresetID(nil), presetOrder...)
}

// Preset returns the catalog layout for an ID. The returned Layout does not alias the catalog.
func Preset(id PresetID) (Layout, bool) {
	layout, ok := presets[id]
	return layout, ok
}
