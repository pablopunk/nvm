package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
)

func TestLargeViewsGetThePreviewWindow(t *testing.T) {
	cases := []struct {
		name string
		view api.View
		want palette.Presentation
	}{
		{"default size stacks", api.View{}, palette.Stacked},
		{"large size takes the large window", api.View{Size: api.SizeLarge}, palette.Preview},
		{"preview takes the large window", api.View{Presentation: api.PresentPreview}, palette.Preview},
		{"side preview keeps its own size", api.View{Size: api.SizeLarge, Presentation: api.PresentSidePreview}, palette.SidePreview},
	}
	for _, tc := range cases {
		if got := palettePresentation(tc.view); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
