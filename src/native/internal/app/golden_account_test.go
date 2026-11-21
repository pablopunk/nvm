package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/design/golden"
	palaccount "github.com/pablopunk/nvm/native/internal/palette/account"
)

// TestAccountPageGoldens renders the account page in every sign-in state the fixtures list: signed out, each
// phase of the device flow, signed in, with a BYO key, and with the spend-cap notice.
func TestAccountPageGoldens(t *testing.T) {
	for _, state := range palaccount.States() {
		p := goldenPalette(t)
		snapshot := state.Snapshot
		goldenOpen(p, palaccount.View(func() auth.Snapshot { return snapshot }, palaccount.Actions{}))
		golden.AssertMatrix(t, "account-"+state.Name, p.View, paletteGoldens)
	}
}
