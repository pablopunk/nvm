package extview

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestPreviewPageLoadsItsImage checks that a preview view asks the host for its image when it opens, once, so the page
// can draw it, instead of showing only the text.
func TestPreviewPageLoadsItsImage(t *testing.T) {
	var refs []string
	deps := Deps{LoadImage: func(_ context.Context, ref string, _ func(*ui.Bitmap)) { refs = append(refs, ref) }}
	view := api.View{Kind: api.ViewPreview, Title: "Wallpaper", Preview: &api.PreviewSpec{
		Image: &api.Image{Src: "https://w.wallhaven.cc/full/ab/wallhaven-abcdef.jpg"}, Text: "3840x1646",
	}}
	s := NewSession(deps, "test", view)
	s.previewPage()
	s.previewPage()
	if len(refs) != 1 || refs[0] != view.Preview.Image.Src {
		t.Fatalf("image requests = %v, want one for the preview image", refs)
	}
}
