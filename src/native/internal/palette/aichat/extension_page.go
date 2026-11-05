package aichat

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// page is the transcript state the renderer keeps between frames: the scroll of the transcript, and which reasoning
// blocks and tool results the user opened, by the index of their transcript entry.
type page struct {
	deps         renderDeps
	list         ui.ListState
	thinkingOpen map[int]bool
	resultOpen   map[int]bool
}

// renderDeps is what the transcript draws from: the Markdown of a reply, and the clock of its progress rows.
type renderDeps struct {
	Reply func(c *ui.Context, src string, streaming bool)
	Now   func() time.Time
}

// newPage returns the transcript state that draws with deps.
func newPage(deps renderDeps) *page {
	return &page{deps: deps, thinkingOpen: map[int]bool{}, resultOpen: map[int]bool{}}
}
