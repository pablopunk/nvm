package design

import (
	"sync"

	"github.com/egoist/mygo/ui"
)

var (
	iconCache = map[string]*ui.SVG{}
	iconMu    sync.Mutex
)

// Icon returns the Lucide icon called name, parsed once and tinted by the caller's color. An unknown name
// draws the puzzle piece, so a typo shows up on screen rather than as a blank.
func Icon(name string) *ui.SVG {
	iconMu.Lock()
	defer iconMu.Unlock()
	if s, ok := iconCache[name]; ok {
		return s
	}
	src, ok := iconSources[name]
	if !ok {
		src = iconSources["puzzle"]
	}
	s := ui.MustParseSVG([]byte(src))
	iconCache[name] = s
	return s
}
