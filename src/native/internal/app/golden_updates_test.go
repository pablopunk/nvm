package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/palette/updates"
)

// TestUpdatesPageGoldens renders the Updates page in each status the fixtures list, on the channel each one shows.
func TestUpdatesPageGoldens(t *testing.T) {
	for _, state := range updates.States() {
		p := goldenPalette(t)
		goldenOpen(p, updates.View(func() updates.Snapshot { return state }, updates.Actions{
			Check:         func() {},
			Restart:       func() error { return nil },
			ToggleChannel: func() error { return nil },
		}))
		golden.AssertMatrix(t, "updates-"+string(state.State.Status)+"-"+string(state.Channel), p.View, paletteGoldens)
	}
}
