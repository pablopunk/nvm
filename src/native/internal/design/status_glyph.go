package design

import (
	"sync"

	"github.com/egoist/mygo/ui"
)

// statusGlyphSources are the status marks of 1.x's indicator: a ring at a third of its strength, with the mark drawn
// over it in the current color.
var statusGlyphSources = map[string]string{
	"success": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="none" stroke="currentColor"><circle cx="10" cy="10" r="8" stroke-width="1.5" stroke-opacity="0.35"/><path d="M6.5 10.5l2.5 2.5 4.5-5.5" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
	"error":   `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="none" stroke="currentColor"><circle cx="10" cy="10" r="8" stroke-width="1.5" stroke-opacity="0.35"/><path d="M7 7l6 6" stroke-width="1.8" stroke-linecap="round"/><path d="M13 7l-6 6" stroke-width="1.8" stroke-linecap="round"/></svg>`,
}

var (
	statusGlyphCache = map[string]*ui.SVG{}
	statusGlyphMu    sync.Mutex
)

// StatusGlyph returns the status mark called kind ("success" or "error"), parsed once and tinted by the caller's color.
// An unknown kind draws nothing, so a status that has no mark shows as plain text.
func StatusGlyph(kind string) *ui.SVG {
	statusGlyphMu.Lock()
	defer statusGlyphMu.Unlock()
	if s, ok := statusGlyphCache[kind]; ok {
		return s
	}
	src, ok := statusGlyphSources[kind]
	if !ok {
		return nil
	}
	s := ui.MustParseSVG([]byte(src))
	statusGlyphCache[kind] = s
	return s
}
