package api

// Tone is the semantic colour of an accessory or metadata row.
type Tone string

const (
	ToneDefault Tone = "default"
	ToneMuted   Tone = "muted"
	ToneAccent  Tone = "accent"
	ToneSuccess Tone = "success"
	ToneWarning Tone = "warning"
	ToneDanger  Tone = "danger"
)

// Color is the muted semantic foreground applied to an item title and its icon.
type Color string

const (
	ColorYellow Color = "yellow"
	ColorBlue   Color = "blue"
	ColorPurple Color = "purple"
	ColorGreen  Color = "green"
	ColorRed    Color = "red"
	ColorOrange Color = "orange"
	ColorPink   Color = "pink"
)

// PanelVisibility controls whether an action panel is shown, menu-only, or hidden while its
// shortcuts keep working.
type PanelVisibility string

const (
	PanelVisible PanelVisibility = "visible"
	PanelMenu    PanelVisibility = "menu"
	PanelHidden  PanelVisibility = "hidden"
)

// PatchMode says how ViewPatch.Items are applied to the visible view.
type PatchMode string

const (
	PatchMerge   PatchMode = "patch"
	PatchReplace PatchMode = "replace"
	PatchPrepend PatchMode = "prepend"
	PatchAppend  PatchMode = "append"
)

// BackgroundMode is the execution lifecycle of a command or menu bar job.
type BackgroundMode string

const (
	ModeView       BackgroundMode = "view"
	ModeNoView     BackgroundMode = "noView"
	ModeBackground BackgroundMode = "background"
)

// Placement says where a command's row appears. No placement means the root and search, as before. Root lists the row
// at the empty root too. Search lists it for a typed query only. Hidden lists it nowhere: it runs from a shortcut or
// an alias. A command with several placements is listed where any of them says.
type Placement string

const (
	PlacementSearch Placement = "search"
	PlacementRoot   Placement = "root"
	PlacementHidden Placement = "hidden"
)

// NavigationMode is how a handler's Result changes the palette navigation stack.
type NavigationMode string

const (
	NavRoot    NavigationMode = "root"
	NavPush    NavigationMode = "push"
	NavReplace NavigationMode = "replace"
	NavPop     NavigationMode = "pop"
)

// ImageFit, ImageShape and ImageMask style an Image.
type (
	ImageFit   string
	ImageShape string
	ImageMask  string
)

const (
	FitCover   ImageFit = "cover"
	FitContain ImageFit = "contain"

	ShapeSquare  ImageShape = "square"
	ShapeRounded ImageShape = "rounded"
	ShapeCircle  ImageShape = "circle"

	MaskNone    ImageMask = "none"
	MaskRounded ImageMask = "rounded"
	MaskCircle  ImageMask = "circle"
)

// oneOf reports whether value is one of allowed. It keeps every enum check a single line.
func oneOf[T ~string](value T, allowed ...T) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
