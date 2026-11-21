package palette

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestPlaceOnWorkAreaCentersAndPlacesLow(t *testing.T) {
	work := platform.Rect{X: 0, Y: 25, Width: 1440, Height: 875}
	x, y := PlaceOnWorkArea(work, 720, 470)
	if x != 360 || y != 25+157 {
		t.Fatalf("placed at %d,%d; want 360,182 (a fifth of the work area down, centered)", x, y)
	}
}

func TestPlaceOnWorkAreaCapsTheTopOffsetOnTallDisplays(t *testing.T) {
	work := platform.Rect{X: 0, Y: 0, Width: 2560, Height: 2000}
	if _, y := PlaceOnWorkArea(work, 720, 470); y != placementMaxTop {
		t.Fatalf("top = %d, want the %d DIP cap", y, placementMaxTop)
	}
}

func TestPlaceOnWorkAreaFollowsTheDisplayOrigin(t *testing.T) {
	work := platform.Rect{X: -1920, Y: 0, Width: 1920, Height: 1080}
	x, _ := PlaceOnWorkArea(work, 720, 470)
	if x != -1920+600 {
		t.Fatalf("x = %d, want the window centered on the display to the left of the primary", x)
	}
}

func TestPlaceOnWorkAreaKeepsAWindowLargerThanTheAreaOnItsEdge(t *testing.T) {
	work := platform.Rect{X: 100, Y: 50, Width: 500, Height: 300}
	x, y := PlaceOnWorkArea(work, 720, 470)
	if x != 100 || y != 50 {
		t.Fatalf("placed at %d,%d; want the low corner 100,50", x, y)
	}
}
