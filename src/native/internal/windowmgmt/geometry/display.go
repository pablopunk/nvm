package geometry

import (
	"math"
	"sort"
)

// minDisplaysToMove is the number of displays needed before a window can change display.
const minDisplaysToMove = 2

// intersectionArea is the overlap area of two rectangles, zero when they do not overlap.
func intersectionArea(a, b Rect) float64 {
	width := math.Min(a.X+a.Width, b.X+b.Width) - math.Max(a.X, b.X)
	height := math.Min(a.Y+a.Height, b.Y+b.Height) - math.Max(a.Y, b.Y)
	if width > 0 && height > 0 {
		return width * height
	}
	return 0
}

func primaryDisplayOf(displays []Display) (Display, bool) {
	for _, display := range displays {
		if display.Main {
			return display, true
		}
	}
	return Display{}, false
}

// PickDisplayForFrame returns the display a frame belongs to: the one with the largest overlap.
// A frame that overlaps no display falls back to the main display, then the first display. On an
// exact tie the main display wins, regardless of list order.
func PickDisplayForFrame(displays []Display, frame Rect) (Display, error) {
	if len(displays) == 0 {
		return Display{}, ErrNoDisplay
	}
	fallback, hasMain := primaryDisplayOf(displays)
	if !hasMain {
		fallback = displays[0]
	}
	largest := 0.0
	for _, display := range displays {
		largest = math.Max(largest, intersectionArea(display.WorkArea, frame))
	}
	if largest == 0 {
		return fallback, nil
	}
	var largestDisplays []Display
	for _, display := range displays {
		if intersectionArea(display.WorkArea, frame) == largest {
			largestDisplays = append(largestDisplays, display)
		}
	}
	if main, ok := primaryDisplayOf(largestDisplays); ok {
		return main, nil
	}
	return largestDisplays[0], nil
}

// RestoreDisplay picks the display a restore should land on: the one holding the stored frame.
func RestoreDisplay(displays []Display, previous *History) (Display, error) {
	if previous == nil || previous.RestoreFrame == nil {
		return Display{}, ErrNoHistory
	}
	return PickDisplayForFrame(displays, *previous.RestoreFrame)
}

func compareLeftToRight(a, b Display) int {
	if a.Frame.X != b.Frame.X {
		return compareFloats(a.Frame.X, b.Frame.X)
	}
	return compareFloats(a.Frame.Y, b.Frame.Y)
}

func compareTopToBottom(a, b Display) int {
	if a.Frame.Y != b.Frame.Y {
		return compareFloats(a.Frame.Y, b.Frame.Y)
	}
	return compareFloats(a.Frame.X, b.Frame.X)
}

func compareFloats(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// AdjacentDisplay returns the display after or before from in the given order, wrapping around.
// It reports false when there are fewer than two displays or from is not in the list.
func AdjacentDisplay(displays []Display, from Display, direction Direction, order DisplayOrder) (Display, bool) {
	if len(displays) < minDisplaysToMove {
		return Display{}, false
	}
	ordered := append([]Display(nil), displays...)
	compare := compareLeftToRight
	if order == DisplayOrderTopToBottom {
		compare = compareTopToBottom
	}
	sort.SliceStable(ordered, func(i, j int) bool { return compare(ordered[i], ordered[j]) < 0 })
	currentIndex := -1
	for index, display := range ordered {
		if display.ID == from.ID {
			currentIndex = index
			break
		}
	}
	if currentIndex == -1 {
		return Display{}, false
	}
	step := -1
	if direction == DirectionNext {
		step = 1
	}
	wrapped := (currentIndex + step + len(ordered)) % len(ordered)
	return ordered[wrapped], true
}

// scaledSpan keeps the window's size relative to the work area, so a window that filled half of
// one display fills half of the other.
func scaledSpan(source, target, windowSpan span) span {
	ratio := target.length / source.length
	return placeSpan(target,
		jsRound(windowSpan.length*ratio),
		target.origin+jsRound((windowSpan.origin-source.origin)*ratio))
}

// keptSizeSpan keeps the window's absolute size and its relative center position.
func keptSizeSpan(source, target, windowSpan span) span {
	length := math.Min(windowSpan.length, target.length)
	sourceCenter := windowSpan.origin + windowSpan.length/2
	relativeCenter := (sourceCenter - source.origin) / source.length
	targetCenter := target.origin + relativeCenter*target.length
	return placeSpan(target, length, targetCenter-length/2)
}

// MoveFrameToDisplay relocates a frame from one display's work area to another's, by proportion
// or by keeping the absolute size. The result is not yet fitted to the target work area.
func MoveFrameToDisplay(frame Rect, from, to Display, sizing DisplayMoveSizing) (Rect, error) {
	if !from.WorkArea.HasArea() || !to.WorkArea.HasArea() {
		return Rect{}, ErrNoWorkArea
	}
	source := axesOf(from.WorkArea)
	target := axesOf(to.WorkArea)
	windowAxes := axesOf(frame)
	relocate := scaledSpan
	if sizing == MoveKeepSize {
		relocate = keptSizeSpan
	}
	return frameFromAxes(axes{
		horizontal: relocate(source.horizontal, target.horizontal, windowAxes.horizontal),
		vertical:   relocate(source.vertical, target.vertical, windowAxes.vertical),
	}), nil
}

// MoveWindowToAdjacentDisplay is the display-move operation: pick the window's display, step to
// the next or previous one, relocate the frame and fit it to the target work area. It returns
// ErrSingleDisplay when there is no other display to move to.
func MoveWindowToAdjacentDisplay(displays []Display, window WindowFrame, direction Direction, options Options) (Rect, Display, error) {
	if len(displays) < minDisplaysToMove {
		return Rect{}, Display{}, ErrSingleDisplay
	}
	from, err := PickDisplayForFrame(displays, window.Frame)
	if err != nil {
		return Rect{}, Display{}, err
	}
	to, ok := AdjacentDisplay(displays, from, direction, options.DisplayOrder)
	if !ok {
		return Rect{}, Display{}, ErrSingleDisplay
	}
	computed, err := MoveFrameToDisplay(window.Frame, from, to, options.DisplayMoveSizing)
	if err != nil {
		return Rect{}, Display{}, err
	}
	return FitRequestedFrame(computed, window, to.WorkArea), to, nil
}
