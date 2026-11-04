package geometry

import (
	"fmt"
	"reflect"
	"testing"
)

// Ported from src/app/electron/os/window-layout-catalog.test.ts.

const catalogEpsilon = 1e-9

var (
	tilingWorkAreas = []Rect{rectOf(0, 0, 1441, 901), rectOf(-1441, -901, 1441, 901)}
	probeWorkArea   = rectOf(0, 0, 1440, 900)
)

// persistedLayoutIDs is the stable ID list; persisted settings refer to these names.
var persistedLayoutIDs = []PresetID{
	"left-half", "right-half", "top-half", "bottom-half", "center-half", "maximize",
	"center", "first-third", "center-third", "last-third", "top-third", "middle-third",
	"bottom-third", "first-two-thirds", "center-two-thirds", "last-two-thirds",
	"top-two-thirds", "middle-two-thirds", "bottom-two-thirds", "top-left-quarter",
	"top-right-quarter", "bottom-left-quarter", "bottom-right-quarter", "first-fourth",
	"second-fourth", "third-fourth", "last-fourth", "top-left-sixth", "top-center-sixth",
	"top-right-sixth", "bottom-left-sixth", "bottom-center-sixth", "bottom-right-sixth",
	"maximize-width", "maximize-height", "almost-maximize", "reasonable-size", "move-left",
	"move-right", "move-up", "move-down", "make-larger", "make-smaller", "next-display",
	"previous-display", "restore", "toggle-full-screen",
}

type tilingGroup struct {
	name      string
	layoutIDs []PresetID
}

var tilingGroups = []tilingGroup{
	{"first, center and last third", []PresetID{"first-third", "center-third", "last-third"}},
	{"top, middle and bottom third", []PresetID{"top-third", "middle-third", "bottom-third"}},
	{"four fourths", []PresetID{"first-fourth", "second-fourth", "third-fourth", "last-fourth"}},
	{"six sixths", []PresetID{"top-left-sixth", "top-center-sixth", "top-right-sixth", "bottom-left-sixth", "bottom-center-sixth", "bottom-right-sixth"}},
	{"four quarters", []PresetID{"top-left-quarter", "top-right-quarter", "bottom-left-quarter", "bottom-right-quarter"}},
}

func overlapArea(a, b Rect) float64 {
	width := min(a.X+a.Width, b.X+b.Width) - max(a.X, b.X)
	height := min(a.Y+a.Height, b.Y+b.Height) - max(a.Y, b.Y)
	if width > 0 && height > 0 {
		return width * height
	}
	return 0
}

func isRectInside(frame, area Rect) bool {
	return frame.X >= area.X && frame.Y >= area.Y &&
		frame.X+frame.Width <= area.X+area.Width && frame.Y+frame.Height <= area.Y+area.Height
}

func tiledFramesFor(t *testing.T, ids []PresetID, workArea Rect) []Rect {
	t.Helper()
	frames := make([]Rect, 0, len(ids))
	for _, id := range ids {
		frames = append(frames, presetFrame(t, id, workArea, 0, probeWindow))
	}
	return frames
}

func totalArea(frames []Rect) float64 {
	sum := 0.0
	for _, frame := range frames {
		sum += frame.Width * frame.Height
	}
	return sum
}

func totalPairwiseOverlap(frames []Rect) float64 {
	sum := 0.0
	for i := range frames {
		for j := i + 1; j < len(frames); j++ {
			sum += overlapArea(frames[i], frames[j])
		}
	}
	return sum
}

func TestPersistedLayoutIDsArePortedVerbatim(t *testing.T) {
	ids := PresetIDs()
	seen := map[PresetID]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate preset id %q", id)
		}
		seen[id] = true
	}
	if !reflect.DeepEqual(ids, persistedLayoutIDs) {
		t.Fatalf("PresetIDs() = %v, want %v", ids, persistedLayoutIDs)
	}
}

func TestEveryCatalogSpecIsAcceptedByTheEngineOrIsServiceHandled(t *testing.T) {
	probe := resizableWindow(probeWindow)
	for _, id := range PresetIDs() {
		layout := mustPreset(t, id)
		frame, _, err := Resolve(layout, displayOver("probe", probeWorkArea), probe, UniformGaps(0), nil)
		if IsServiceHandled(layout.Kind) {
			if err == nil {
				t.Fatalf("%s: service-handled layout resolved without error", id)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if !finite(frame.X) || !finite(frame.Width) {
			t.Fatalf("%s: non-finite frame %+v", id, frame)
		}
	}
}

func TestServiceHandledCatalogSpecsAreExactlyRestoreDisplayAndFullScreen(t *testing.T) {
	var serviceHandled []PresetID
	for _, id := range PresetIDs() {
		if IsServiceHandled(mustPreset(t, id).Kind) {
			serviceHandled = append(serviceHandled, id)
		}
	}
	want := []PresetID{"next-display", "previous-display", "restore", "toggle-full-screen"}
	if !reflect.DeepEqual(serviceHandled, want) {
		t.Fatalf("service handled = %v, want %v", serviceHandled, want)
	}
}

func TestEveryFractionSpecLiesInsideTheUnitSquare(t *testing.T) {
	for _, id := range PresetIDs() {
		layout := mustPreset(t, id)
		if layout.Kind != KindFraction {
			continue
		}
		if !(layout.X >= 0 && layout.X <= 1) {
			t.Fatalf("%s x = %v", id, layout.X)
		}
		if !(layout.Y >= 0 && layout.Y <= 1) {
			t.Fatalf("%s y = %v", id, layout.Y)
		}
		if !(layout.Width > 0 && layout.Width <= 1) {
			t.Fatalf("%s width = %v", id, layout.Width)
		}
		if !(layout.Height > 0 && layout.Height <= 1) {
			t.Fatalf("%s height = %v", id, layout.Height)
		}
		if layout.X+layout.Width > 1+catalogEpsilon {
			t.Fatalf("%s right edge past 1", id)
		}
		if layout.Y+layout.Height > 1+catalogEpsilon {
			t.Fatalf("%s bottom edge past 1", id)
		}
	}
}

func TestTilingGroupsTileExactly(t *testing.T) {
	for _, group := range tilingGroups {
		for _, workArea := range tilingWorkAreas {
			name := fmt.Sprintf("%s tile a %vx%v work area at %v,%v exactly", group.name, workArea.Width, workArea.Height, workArea.X, workArea.Y)
			t.Run(name, func(t *testing.T) {
				frames := tiledFramesFor(t, group.layoutIDs, workArea)
				if got, want := totalArea(frames), workArea.Width*workArea.Height; got != want {
					t.Fatalf("total area = %v, want %v", got, want)
				}
				if got := totalPairwiseOverlap(frames); got != 0 {
					t.Fatalf("pairwise overlap = %v, want 0", got)
				}
				for _, frame := range frames {
					if !isRectInside(frame, workArea) {
						t.Fatalf("frame %+v outside work area %+v", frame, workArea)
					}
				}
			})
		}
	}
}
