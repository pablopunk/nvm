package windowmgmt

import "context"

// Source is what started a window command. A palette command has to return focus to the app it was
// opened from; a shortcut command must never take focus, so it only checks that the target is still in
// front.
type Source string

const (
	SourcePalette  Source = "palette"
	SourceShortcut Source = "shortcut"
)

// Target is the app a command acts on: the app that was focused when the command started.
type Target struct {
	PID      int
	BundleID string
}

// Invocation is one command start: where it came from and the app it acts on. Target is nil when no
// app was captured, which fails the command with a message for its source.
type Invocation struct {
	Source Source
	Target *Target
}

// FocusHost is what the palette provides: the app it was opened from, and a way to hide itself and hand
// focus back. The engine asks it before a palette command moves a window.
type FocusHost interface {
	// PaletteFocusTarget returns the app the palette was opened from, if it is still known.
	PaletteFocusTarget() (Target, bool)
	// HidePaletteAndRestoreFocus hides the palette and returns focus to target. It reports whether the
	// hand-off was requested; the engine then checks that target is in front.
	HidePaletteAndRestoreFocus(ctx context.Context, target Target) bool
}
