package extwindow

import (
	"encoding/json"
	"fmt"
	"hash/fnv"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Window sizes in points, as the 1.x companion window has them: the default and large presets, and the bounds that
// explicit sizes clamp to.
const (
	defaultWidth, defaultHeight = 560, 420
	largeWidth, largeHeight     = 900, 680
	minWidth, minHeight         = 320, 240
	maxWidth, maxHeight         = 1600, 1200
)

// Size is the width and height a window opens at. An explicit width or height wins over the size preset; zero means
// the preset's value, so a window cannot ask for zero points.
func Size(options api.WindowOptions) (width, height int) {
	presetWidth, presetHeight := defaultWidth, defaultHeight
	if options.Size == api.SizeLarge {
		presetWidth, presetHeight = largeWidth, largeHeight
	}
	return clamp(orValue(options.Width, presetWidth), minWidth, maxWidth),
		clamp(orValue(options.Height, presetHeight), minHeight, maxHeight)
}

// WindowID is the id a window is known by inside its extension: the options' id, else the view's id, else a hash of
// the view. The hash is stable for an unchanged view, so a window opened again finds its own record.
func WindowID(view api.View, options api.WindowOptions) string {
	switch {
	case options.ID != "":
		return options.ID
	case view.ID != "":
		return view.ID
	}
	data, _ := json.Marshal(view)
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(view.Title + ":"))
	_, _ = hash.Write(data)
	return fmt.Sprintf("window:%x", hash.Sum64())
}

func orValue(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
