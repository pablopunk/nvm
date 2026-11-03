package notes

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func display(x, y, w, h int) platform.Display {
	bounds := platform.Rect{X: x, Y: y, Width: w, Height: h}
	return platform.Display{Bounds: bounds, WorkArea: bounds, ScaleFactor: 1}
}

// The window restore of the 1.x host (extension-window-manager clampedFrame) and its display recovery, ported as
// table cases: a frame stays where it is on a connected display, moves onto the display it was nearest to when its
// display is gone, shrinks to a smaller work area, and is returned unchanged when no display is known.
func TestFitFrameKeepsOrMovesARememberedFrame(t *testing.T) {
	main := display(0, 0, 1920, 1080)
	left := display(-1280, 0, 1280, 1024)
	cases := []struct {
		name     string
		saved    Frame
		displays []platform.Display
		want     Frame
	}{
		{
			name:     "on a connected display it stays",
			saved:    Frame{X: 100, Y: 120, Width: 640, Height: 460},
			displays: []platform.Display{main},
			want:     Frame{X: 100, Y: 120, Width: 640, Height: 460},
		},
		{
			name:     "display gone to the right moves onto the only one",
			saved:    Frame{X: 3000, Y: 200, Width: 640, Height: 460},
			displays: []platform.Display{main},
			want:     Frame{X: 1280, Y: 200, Width: 640, Height: 460},
		},
		{
			name:     "partly off the bottom is pulled inside",
			saved:    Frame{X: 100, Y: 900, Width: 640, Height: 460},
			displays: []platform.Display{main},
			want:     Frame{X: 100, Y: 620, Width: 640, Height: 460},
		},
		{
			name:     "larger than the work area shrinks to it",
			saved:    Frame{X: 0, Y: 0, Width: 2000, Height: 1200},
			displays: []platform.Display{main},
			want:     Frame{X: 0, Y: 0, Width: 1920, Height: 1080},
		},
		{
			name:     "a frame on a display that is still there keeps that display",
			saved:    Frame{X: -900, Y: 100, Width: 640, Height: 460},
			displays: []platform.Display{main, left},
			want:     Frame{X: -900, Y: 100, Width: 640, Height: 460},
		},
		{
			name:     "no display known leaves the frame unchanged",
			saved:    Frame{X: 3000, Y: 200, Width: 640, Height: 460},
			displays: nil,
			want:     Frame{X: 3000, Y: 200, Width: 640, Height: 460},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FitFrame(tc.saved, tc.displays)
			if got != tc.want {
				t.Fatalf("FitFrame = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// A frame whose center is on no display goes to the display nearest its center.
func TestFitFrameUsesTheNearestDisplayWhenCenterIsOffEveryDisplay(t *testing.T) {
	a := display(0, 0, 1000, 800)
	b := display(1200, 0, 1000, 800)
	got := FitFrame(Frame{X: 1050, Y: 900, Width: 100, Height: 100}, []platform.Display{a, b})
	want := Frame{X: 1200, Y: 700, Width: 100, Height: 100}
	if got != want {
		t.Fatalf("FitFrame = %+v, want %+v", got, want)
	}
}
