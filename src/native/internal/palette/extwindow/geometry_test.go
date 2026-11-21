package extwindow

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestSizeUsesThePresetAndClampsExplicitDimensions(t *testing.T) {
	cases := []struct {
		name          string
		options       api.WindowOptions
		width, height int
	}{
		{"default preset", api.WindowOptions{}, 560, 420},
		{"large preset", api.WindowOptions{Size: api.SizeLarge}, 900, 680},
		{"explicit size wins", api.WindowOptions{Size: api.SizeLarge, Width: 500, Height: 300}, 500, 300},
		{"width below minimum", api.WindowOptions{Width: 100}, 320, 420},
		{"height above maximum", api.WindowOptions{Height: 5000}, 560, 1200},
	}
	for _, c := range cases {
		if width, height := Size(c.options); width != c.width || height != c.height {
			t.Fatalf("%s: Size = %d by %d, want %d by %d", c.name, width, height, c.width, c.height)
		}
	}
}

func TestWindowIDPrefersTheOptionThenTheViewThenAStableHash(t *testing.T) {
	view := api.View{Title: "Ticker"}
	if got := WindowID(view, api.WindowOptions{ID: "explicit"}); got != "explicit" {
		t.Fatalf("option id = %q", got)
	}
	if got := WindowID(api.View{ID: "view-id", Title: "Ticker"}, api.WindowOptions{}); got != "view-id" {
		t.Fatalf("view id = %q", got)
	}
	first, again := WindowID(view, api.WindowOptions{}), WindowID(view, api.WindowOptions{})
	other := WindowID(api.View{Title: "Other"}, api.WindowOptions{})
	if first != again || first == other {
		t.Fatalf("hash ids %q, %q, %q: want stable and distinct", first, again, other)
	}
}

func TestPlaceFrameCentersOrPinsToAnEdge(t *testing.T) {
	area := platform.Rect{X: 0, Y: 0, Width: 1000, Height: 800}
	if got := PlaceFrame(area, 200, 100, "center"); got != (Frame{X: 400, Y: 350, Width: 200, Height: 100}) {
		t.Fatalf("center = %+v", got)
	}
	if got := PlaceFrame(area, 200, 100, "top-center"); got.Y != placementMargin || got.X != 400 {
		t.Fatalf("top-center = %+v", got)
	}
	if got := PlaceFrame(area, 200, 100, "bottom-center"); got.Y != 800-100-placementMargin {
		t.Fatalf("bottom-center = %+v", got)
	}
}

func TestFitFrameMovesAFrameSavedOnAMissingDisplayOntoTheConnectedOne(t *testing.T) {
	displays := []platform.Display{{WorkArea: platform.Rect{X: 0, Y: 0, Width: 1000, Height: 800}, Bounds: platform.Rect{X: 0, Y: 0, Width: 1000, Height: 800}}}
	got := FitFrame(Frame{X: 2000, Y: 100, Width: 400, Height: 300}, displays)
	if got != (Frame{X: 600, Y: 100, Width: 400, Height: 300}) {
		t.Fatalf("fit = %+v, want the frame moved inside the work area", got)
	}
}
