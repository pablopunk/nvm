package app

import (
	"github.com/pablopunk/nvm/native/internal/platform"
	"testing"
)

func TestIndicatorExpansionKeepsItsBottomAnchor(t *testing.T) {
	area := platform.Rect{X: 100, Y: 50, Width: 1200, Height: 800}
	for _, size := range [][2]int{{224, 52}, {320, 124}} {
		x, y := aiPanelOrigin(area, size[0], size[1])
		if x+size[0]/2 != area.X+area.Width/2 || y+size[1] != area.Y+area.Height-aiPanelBottomGap {
			t.Fatalf("indicator moved away from its anchor at %v: %d,%d", size, x, y)
		}
	}
}
