package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/palette"
)

func TestExtensionStateRefreshesTheRootAndTheExtensionsList(t *testing.T) {
	for _, title := range []string{rootViewTitle, extensionsViewTitle} {
		if !isExtensionStateView(&palette.View{Title: title}) {
			t.Errorf("%q does not refresh on extension state", title)
		}
	}
	if isExtensionStateView(&palette.View{Title: "Settings"}) {
		t.Error("Settings refreshes on extension state")
	}
}
