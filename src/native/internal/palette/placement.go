package palette

import "github.com/pablopunk/nvm/native/internal/platform"

// The palette sits a fifth of the way down its display, and never lower than this many DIPs (Electron's
// paletteBoundsPlacedInWorkArea).
const (
	placementHeightPercent = 18
	placementMaxTop        = 180
)

// PlaceOnWorkArea returns the top left corner of a w by h palette window on a display's work area: centered
// horizontally, placed low on the work area's top part, and kept inside the work area.
func PlaceOnWorkArea(work platform.Rect, w, h int) (x, y int) {
	top := min(work.Height*placementHeightPercent/100, placementMaxTop)
	return clampInto(work.X+(work.Width-w)/2, work.X, work.X+work.Width-w),
		clampInto(work.Y+top, work.Y, work.Y+work.Height-h)
}

// clampInto keeps v within [low, high]. A window larger than its area is pinned to the low edge.
func clampInto(v, low, high int) int {
	if high < low {
		return low
	}
	return min(max(v, low), high)
}
