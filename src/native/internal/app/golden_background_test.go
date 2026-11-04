package app

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/palette/background"
)

// backgroundNow anchors the times the Background Tasks goldens show. It is the fixture epoch, so every relative time
// reads the same on every run.
var backgroundNow = time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)

// TestPaletteBackgroundTasksList renders the list with one row per state, and the action menu of the failed job.
func TestPaletteBackgroundTasksList(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, background.View(background.Fixture(backgroundNow), func() time.Time { return backgroundNow }))
	golden.AssertMatrix(t, "background-list", p.View, paletteGoldens)

	p = goldenPalette(t)
	goldenOpen(p, background.View(background.Fixture(backgroundNow), func() time.Time { return backgroundNow }))
	p.Choose(1)
	p.OpenActionMenu(1)
	golden.AssertMatrix(t, "background-actions", p.View, paletteGoldens)
}

// TestPaletteBackgroundTasksHistory renders the run history of the failed job.
func TestPaletteBackgroundTasksHistory(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, background.Details(background.Fixture(backgroundNow), "extension.acme.feed.fetch"))
	golden.AssertMatrix(t, "background-history", p.View, paletteGoldens)
}
