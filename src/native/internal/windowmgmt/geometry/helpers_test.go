package geometry

import (
	"testing"
)

// rectOf is shorthand for a Rect literal in tables.
func rectOf(x, y, width, height float64) Rect {
	return Rect{X: x, Y: y, Width: width, Height: height}
}

// resizableWindow is a window that can resize, showing frame.
func resizableWindow(frame Rect) WindowFrame {
	return WindowFrame{Frame: frame, IsResizable: true}
}

// displayOver is a display whose frame and work area are both area.
func displayOver(id string, area Rect) Display {
	return Display{ID: id, Scale: 1, Frame: area, WorkArea: area}
}

// mustPreset returns a catalog layout or fails the test.
func mustPreset(t *testing.T, id PresetID) Layout {
	t.Helper()
	layout, ok := Preset(id)
	if !ok {
		t.Fatalf("preset %q is missing from the catalog", id)
	}
	return layout
}

// targetFrame resolves a layout on a work area for a resizable window, as the TypeScript
// computeTargetFrame tests do (no fitting is involved for resizable windows).
func targetFrame(t *testing.T, layout Layout, workArea Rect, options Options, current Rect) Rect {
	t.Helper()
	frame, _, err := Resolve(layout, displayOver("test", workArea), resizableWindow(current), options, nil)
	if err != nil {
		t.Fatalf("Resolve(%+v): %v", layout, err)
	}
	return frame
}

// presetFrame resolves a catalog preset on a work area with uniform gaps.
func presetFrame(t *testing.T, id PresetID, workArea Rect, gap float64, current Rect) Rect {
	t.Helper()
	return targetFrame(t, mustPreset(t, id), workArea, UniformGaps(gap), current)
}

func assertFrame(t *testing.T, got, want Rect) {
	t.Helper()
	if got != want {
		t.Fatalf("frame = %+v, want %+v", got, want)
	}
}
