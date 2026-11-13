package geometry

import (
	"fmt"
	"testing"
)

// Ported from 1.x/electron/os/window-layouts.test.ts.

var primaryWorkArea = rectOf(0, 0, 1440, 900)
var probeWindow = rectOf(100, 120, 800, 500)

func TestPresetFramesOnPrimaryDisplayPort(t *testing.T) {
	cases := []struct {
		layoutID PresetID
		gap      float64
		expected Rect
	}{
		{"left-half", 0, rectOf(0, 0, 720, 900)},
		{"right-half", 0, rectOf(720, 0, 720, 900)},
		{"top-half", 0, rectOf(0, 0, 1440, 450)},
		{"bottom-half", 0, rectOf(0, 450, 1440, 450)},
		{"maximize", 0, rectOf(0, 0, 1440, 900)},
		{"left-half", 10, rectOf(10, 10, 705, 880)},
		{"right-half", 10, rectOf(725, 10, 705, 880)},
		{"top-half", 10, rectOf(10, 10, 1420, 435)},
		{"bottom-half", 10, rectOf(10, 455, 1420, 435)},
		{"maximize", 10, rectOf(10, 10, 1420, 880)},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s on a 1440x900 origin display with gap %v", tc.layoutID, tc.gap), func(t *testing.T) {
			assertFrame(t, presetFrame(t, tc.layoutID, primaryWorkArea, tc.gap, probeWindow), tc.expected)
		})
	}
}

func TestCenterLayoutPort(t *testing.T) {
	t.Run("center keeps the window size and centers it in the work area", func(t *testing.T) {
		assertFrame(t, presetFrame(t, "center", primaryWorkArea, 0, probeWindow), rectOf(320, 200, 800, 500))
	})
	t.Run("center ignores the gap", func(t *testing.T) {
		got := presetFrame(t, "center", primaryWorkArea, 10, probeWindow)
		assertFrame(t, got, presetFrame(t, "center", primaryWorkArea, 0, probeWindow))
	})
	t.Run("center rounds the origin when the leftover space is odd", func(t *testing.T) {
		oddWindow := rectOf(0, 0, 801, 501)
		assertFrame(t, presetFrame(t, "center", primaryWorkArea, 0, oddWindow), rectOf(320, 200, 801, 501))
	})
	t.Run("center clamps a window larger than the work area to the work area", func(t *testing.T) {
		oversized := rectOf(0, 0, 2000, 1200)
		assertFrame(t, presetFrame(t, "center", primaryWorkArea, 0, oversized), rectOf(0, 0, 1440, 900))
	})
	t.Run("center clamps an oversized window on a negative-origin display", func(t *testing.T) {
		secondary := rectOf(-1920, 0, 1920, 1080)
		oversized := rectOf(0, 0, 2560, 1440)
		assertFrame(t, presetFrame(t, "center", secondary, 0, oversized), rectOf(-1920, 0, 1920, 1080))
	})
}

func TestSecondaryDisplayCasesPort(t *testing.T) {
	cases := []struct {
		name     string
		workArea Rect
		layoutID PresetID
		gap      float64
		expected Rect
	}{
		{"negative x origin, left half", rectOf(-1920, 0, 1920, 1080), "left-half", 0, rectOf(-1920, 0, 960, 1080)},
		{"negative x origin, right half", rectOf(-1920, 0, 1920, 1080), "right-half", 0, rectOf(-960, 0, 960, 1080)},
		{"negative x origin, left half with gap 10", rectOf(-1920, 0, 1920, 1080), "left-half", 10, rectOf(-1910, 10, 945, 1060)},
		{"display above the primary, maximize", rectOf(0, -1080, 1920, 1080), "maximize", 0, rectOf(0, -1080, 1920, 1080)},
		{"display above the primary, bottom half", rectOf(0, -1080, 1920, 1080), "bottom-half", 0, rectOf(0, -540, 1920, 540)},
		{"display above the primary, top half", rectOf(0, -1080, 1920, 1080), "top-half", 0, rectOf(0, -1080, 1920, 540)},
		{"portrait display left of the primary, top half", rectOf(-1080, 0, 1080, 1920), "top-half", 0, rectOf(-1080, 0, 1080, 960)},
		{"portrait display left of the primary, left half", rectOf(-1080, 0, 1080, 1920), "left-half", 0, rectOf(-1080, 0, 540, 1920)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFrame(t, presetFrame(t, tc.layoutID, tc.workArea, tc.gap, probeWindow), tc.expected)
		})
	}
}

func TestAbuttingHalvesPort(t *testing.T) {
	widths := []float64{1440, 1441, 1919, 1920}
	gaps := []float64{0, 1, 10, 11}
	for _, width := range widths {
		for _, gap := range gaps {
			t.Run(fmt.Sprintf("left and right halves of a %vpx work area with gap %v abut without overlap", width, gap), func(t *testing.T) {
				workArea := rectOf(-width, 0, width, 900)
				left := presetFrame(t, "left-half", workArea, gap, probeWindow)
				right := presetFrame(t, "right-half", workArea, gap, probeWindow)
				if left.X != workArea.X+gap {
					t.Fatalf("left.X = %v, want %v", left.X, workArea.X+gap)
				}
				if right.X+right.Width != workArea.X+workArea.Width-gap {
					t.Fatalf("right edge = %v, want %v", right.X+right.Width, workArea.X+workArea.Width-gap)
				}
				if left.X+left.Width+gap != right.X {
					t.Fatalf("gap between halves = %v, want %v", right.X-(left.X+left.Width), gap)
				}
			})
		}
	}
}

func TestOddWidthHalvesPort(t *testing.T) {
	t.Run("odd-width halves tile the work area exactly without gap", func(t *testing.T) {
		workArea := rectOf(0, 0, 1441, 900)
		assertFrame(t, presetFrame(t, "left-half", workArea, 0, probeWindow), rectOf(0, 0, 721, 900))
		assertFrame(t, presetFrame(t, "right-half", workArea, 0, probeWindow), rectOf(721, 0, 720, 900))
	})
	t.Run("odd-width halves keep exactly the gap between them", func(t *testing.T) {
		workArea := rectOf(0, 0, 1441, 900)
		assertFrame(t, presetFrame(t, "left-half", workArea, 10, probeWindow), rectOf(10, 10, 706, 880))
		assertFrame(t, presetFrame(t, "right-half", workArea, 10, probeWindow), rectOf(726, 10, 705, 880))
	})
}

var (
	displayPrimary = Display{ID: "primary", Main: true, Frame: primaryWorkArea, WorkArea: primaryWorkArea}
	displayRight   = displayOver("right", rectOf(1440, 0, 1920, 1080))
	displayLeftNeg = displayOver("left", rectOf(-1920, 0, 1920, 1080))
	displayAbove   = displayOver("above", rectOf(0, -1080, 1920, 1080))
	straddling     = rectOf(1000, 0, 1000, 500)
	offScreen      = rectOf(5000, 5000, 400, 300)
)

func TestPickDisplayPort(t *testing.T) {
	t.Run("picks the display with the largest overlap for a window straddling two displays", func(t *testing.T) {
		got, err := PickDisplayForFrame([]Display{displayPrimary, displayRight}, straddling)
		if err != nil || got.ID != displayRight.ID {
			t.Fatalf("got %v, %v; want %s", got.ID, err, displayRight.ID)
		}
	})
	t.Run("picks the negative-origin display when the window sits on it", func(t *testing.T) {
		frame := rectOf(-1500, 100, 600, 400)
		got, err := PickDisplayForFrame([]Display{displayPrimary, displayLeftNeg}, frame)
		if err != nil || got.ID != displayLeftNeg.ID {
			t.Fatalf("got %v, %v; want %s", got.ID, err, displayLeftNeg.ID)
		}
	})
	t.Run("falls back to the primary display for a fully off-screen window", func(t *testing.T) {
		got, err := PickDisplayForFrame([]Display{displayRight, displayPrimary, displayAbove}, offScreen)
		if err != nil || got.ID != displayPrimary.ID {
			t.Fatalf("got %v, %v; want primary", got.ID, err)
		}
	})
	t.Run("falls back to the first display when none is primary and the window is off-screen", func(t *testing.T) {
		withoutPrimary := []Display{displayOver("right", displayRight.WorkArea), displayOver("above", displayAbove.WorkArea)}
		got, err := PickDisplayForFrame(withoutPrimary, offScreen)
		if err != nil || got.ID != withoutPrimary[0].ID {
			t.Fatalf("got %v, %v; want first display", got.ID, err)
		}
	})
	t.Run("returns the only display for any frame", func(t *testing.T) {
		for _, frame := range []Rect{offScreen, straddling} {
			got, err := PickDisplayForFrame([]Display{displayPrimary}, frame)
			if err != nil || got.ID != displayPrimary.ID {
				t.Fatalf("frame %+v: got %v, %v", frame, got.ID, err)
			}
		}
	})
	t.Run("prefers the primary display on an exact overlap tie regardless of list order", func(t *testing.T) {
		other := displayOver("other", rectOf(1000, 0, 1000, 800))
		tieFrame := rectOf(500, 0, 1000, 800)
		primaryTie := Display{ID: "primary-tie", Main: true, Frame: rectOf(0, 0, 1000, 800), WorkArea: rectOf(0, 0, 1000, 800)}
		for _, order := range [][]Display{{other, primaryTie}, {primaryTie, other}} {
			got, err := PickDisplayForFrame(order, tieFrame)
			if err != nil || got.ID != primaryTie.ID {
				t.Fatalf("order %v: got %v, %v; want primary-tie", order, got.ID, err)
			}
		}
	})
	t.Run("throws when there are no displays", func(t *testing.T) {
		if _, err := PickDisplayForFrame(nil, straddling); err == nil {
			t.Fatal("expected an error with no displays")
		}
	})
}
