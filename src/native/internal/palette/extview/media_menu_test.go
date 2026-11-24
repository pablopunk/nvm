package extview

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
)

func TestCameraPageOffersTheExtensionActionsInCommandK(t *testing.T) {
	view := cameraFixtureView
	view.Actions = []api.Action{api.Run("next-camera", "Next Camera", nil)}
	view.FooterHints = []api.FooterHint{{Key: "n", Title: "Next"}}
	rig := newMediaRig(t, view)
	assertMenuLists(t, rig.page, "Next Camera")
	if rig.page.Hint == "" {
		t.Fatal("the camera page drops the view's footer hints")
	}
}

func TestWebviewPageOffersTheExtensionActionsInCommandK(t *testing.T) {
	view := api.View{ID: "webview-fixture", Kind: api.ViewWebview, Title: "Webview fixture", Webview: &api.WebviewSpec{HTML: "<p>x</p>"}}
	view.Actions = []api.Action{api.Run("reload", "Reload Page", nil)}
	rig := newMediaRig(t, view)
	assertMenuLists(t, rig.page, "Reload Page")
}

func assertMenuLists(t *testing.T, page *palette.View, title string) {
	t.Helper()
	if page.Actions == nil {
		t.Fatalf("the page has no Command-K menu, want %q in it", title)
	}
	var titles []string
	for _, action := range page.Actions() {
		titles = append(titles, action.Title)
	}
	if !slices.Contains(titles, title) {
		t.Fatalf("menu = %q, want %q in it", titles, title)
	}
}
