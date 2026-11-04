package fake

import (
	"math"
	"slices"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// placedFrame is the frame a simulated window reaches for a write: the origin always moves, a resizable
// window takes the requested size but never less than its minimum, and a fixed or position-only write
// keeps the current size.
func placedFrame(entry *simWindow, write platform.WindowWrite) platform.WindowFrame {
	if write.PositionOnly || entry.Fixed {
		return platform.WindowFrame{X: write.Frame.X, Y: write.Frame.Y, Width: entry.Frame.Width, Height: entry.Frame.Height}
	}
	return platform.WindowFrame{
		X:      write.Frame.X,
		Y:      write.Frame.Y,
		Width:  math.Max(write.Frame.Width, entry.MinWidth),
		Height: math.Max(write.Frame.Height, entry.MinHeight),
	}
}

// displayOfLocked returns the display with the largest overlap with frame, or the first display when
// none overlaps. It returns the zero display when there are no displays.
func (w *WindowManager) displayOfLocked(frame platform.WindowFrame) platform.WindowDisplay {
	var best platform.WindowDisplay
	bestArea := -1.0
	for _, display := range w.displays {
		if area := overlapArea(display.WorkArea, frame); area > bestArea {
			best, bestArea = display, area
		}
	}
	if bestArea == 0 && len(w.displays) > 0 {
		return w.displays[0]
	}
	return best
}

func overlapArea(area, frame platform.WindowFrame) float64 {
	width := math.Min(area.X+area.Width, frame.X+frame.Width) - math.Max(area.X, frame.X)
	height := math.Min(area.Y+area.Height, frame.Y+frame.Height) - math.Max(area.Y, frame.Y)
	if width <= 0 || height <= 0 {
		return 0
	}
	return width * height
}

// ownedLocked returns the window only when it still exists and belongs to pid; otherwise the target is gone.
func (w *WindowManager) ownedLocked(pid int, key string) (*simWindow, error) {
	entry, ok := w.windows[key]
	if !ok || entry.PID != pid {
		return nil, &platform.WindowError{Code: platform.WindowErrorTargetLost}
	}
	return entry, nil
}

func (w *WindowManager) frontLocked() *simWindow {
	if len(w.stack) == 0 {
		return nil
	}
	return w.windows[w.stack[0]]
}

func (w *WindowManager) topmostOfLocked(pid int) *simWindow {
	for _, key := range w.stack {
		if entry := w.windows[key]; entry != nil && entry.PID == pid {
			return entry
		}
	}
	return nil
}

func (w *WindowManager) bringToFrontLocked(key string) {
	if _, ok := w.windows[key]; !ok {
		return
	}
	w.stack = append([]string{key}, removeKey(w.stack, key)...)
}

// recordsLocked lists the matching windows front to back, with ZIndex set from their stack position.
func (w *WindowManager) recordsLocked(keep func(*simWindow) bool) []platform.WindowRecord {
	records := []platform.WindowRecord{}
	for index, key := range w.stack {
		entry := w.windows[key]
		if entry == nil || !keep(entry) {
			continue
		}
		record := w.recordLocked(entry)
		record.ZIndex = index
		records = append(records, record)
	}
	return records
}

func (w *WindowManager) recordLocked(entry *simWindow) platform.WindowRecord {
	display := w.displayOfLocked(entry.Frame)
	return platform.WindowRecord{
		Key:        entry.Key,
		PID:        entry.PID,
		BundleID:   entry.BundleID,
		AppName:    entry.AppName,
		Title:      entry.Title,
		Frame:      entry.Frame,
		DisplayID:  display.ID,
		FullScreen: entry.FullScreen,
		Minimized:  entry.Minimized,
		Movable:    !entry.Fixed,
		Resizable:  !entry.Fixed,
	}
}

// appsLocked lists the owners of the stacked windows, front-most app first.
func (w *WindowManager) appsLocked() []platform.WindowApp {
	seen := map[int]bool{}
	apps := []platform.WindowApp{}
	for _, key := range w.stack {
		entry := w.windows[key]
		if entry == nil || seen[entry.PID] {
			continue
		}
		seen[entry.PID] = true
		apps = append(apps, platform.WindowApp{PID: entry.PID, BundleID: entry.BundleID, Name: entry.AppName})
	}
	return apps
}

func removeKey(keys []string, key string) []string {
	return slices.DeleteFunc(slices.Clone(keys), func(candidate string) bool { return candidate == key })
}
