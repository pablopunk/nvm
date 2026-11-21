package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestRunPreviewActionSuppressesTheGenericPreview checks that a row whose extension runs its own Preview is not given a
// second Preview by the host.
func TestRunPreviewActionSuppressesTheGenericPreview(t *testing.T) {
	item := api.Item{
		ID:      "shot",
		Title:   "shot.png",
		Image:   &api.Image{Src: "file:///tmp/shot.png"},
		Actions: []api.Action{api.Run("preview", api.PreviewTitle, nil), api.CopyText("Copy Path", "/tmp/shot.png", false)},
	}
	if got := WithPreviewActions(item); len(got.Actions) != 2 {
		t.Fatalf("actions %+v; want the extension's own two", got.Actions)
	}
}
