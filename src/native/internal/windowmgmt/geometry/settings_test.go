package geometry

import (
	"reflect"
	"testing"
)

// Ported from 1.x/extensions/window-management-specs.test.ts. The TypeScript cases use the
// settings model's defaults; Settings here carries the same fields and DefaultSettings the same values.

var settingsDependentPresets = map[PresetID]bool{
	"almost-maximize": true, "reasonable-size": true, "center-half": true, "make-larger": true, "make-smaller": true,
}

func layoutFor(t *testing.T, id PresetID, settings Settings) Layout {
	t.Helper()
	layout, ok := LayoutSpecFor(id, settings)
	if !ok {
		t.Fatalf("LayoutSpecFor(%q) found no preset", id)
	}
	return layout
}

func TestLayoutsWithoutSettingsKeepTheirCatalogSpecPort(t *testing.T) {
	for _, id := range PresetIDs() {
		if settingsDependentPresets[id] {
			continue
		}
		t.Run(string(id), func(t *testing.T) {
			if got, want := layoutFor(t, id, DefaultSettings()), mustPreset(t, id); !reflect.DeepEqual(got, want) {
				t.Fatalf("layout = %+v, want %+v", got, want)
			}
		})
	}
}

func TestAlmostMaximizeUsesConfiguredPercentsWithoutCapsPort(t *testing.T) {
	settings := DefaultSettings()
	settings.AlmostMaximizeWidthPercent = 85
	settings.AlmostMaximizeHeightPercent = 60
	want := Layout{Kind: KindScaledCenter, WidthFraction: 0.85, HeightFraction: 0.6}
	if got := layoutFor(t, "almost-maximize", settings); !reflect.DeepEqual(got, want) {
		t.Fatalf("layout = %+v, want %+v", got, want)
	}
}

func TestDefaultAlmostMaximizeIs90PercentPort(t *testing.T) {
	want := Layout{Kind: KindScaledCenter, WidthFraction: 0.9, HeightFraction: 0.9}
	if got := layoutFor(t, "almost-maximize", DefaultSettings()); !reflect.DeepEqual(got, want) {
		t.Fatalf("layout = %+v, want %+v", got, want)
	}
}

func TestReasonableSizeUsesItsOwnPercentsWithNoCapPort(t *testing.T) {
	settings := DefaultSettings()
	settings.ReasonableSizeWidthPercent = 100
	settings.ReasonableSizeHeightPercent = 30
	want := Layout{Kind: KindScaledCenter, WidthFraction: 1, HeightFraction: 0.3}
	if got := layoutFor(t, "reasonable-size", settings); !reflect.DeepEqual(got, want) {
		t.Fatalf("layout = %+v, want %+v", got, want)
	}
}

func TestDefaultReasonableSizeIs70PercentPort(t *testing.T) {
	want := Layout{Kind: KindScaledCenter, WidthFraction: 0.7, HeightFraction: 0.7}
	if got := layoutFor(t, "reasonable-size", DefaultSettings()); !reflect.DeepEqual(got, want) {
		t.Fatalf("layout = %+v, want %+v", got, want)
	}
}

func TestCenterHalfKeepsConfiguredWidthAndCentersItPort(t *testing.T) {
	settings := DefaultSettings()
	settings.CenterHalfWidthPercent = 50
	if got, want := layoutFor(t, "center-half", settings), (Layout{Kind: KindFraction, X: 0.25, Y: 0, Width: 0.5, Height: 1}); !reflect.DeepEqual(got, want) {
		t.Fatalf("50%%: layout = %+v, want %+v", got, want)
	}
	settings.CenterHalfWidthPercent = 60
	if got, want := layoutFor(t, "center-half", settings), (Layout{Kind: KindFraction, X: 0.2, Y: 0, Width: 0.6, Height: 1}); !reflect.DeepEqual(got, want) {
		t.Fatalf("60%%: layout = %+v, want %+v", got, want)
	}
}

func TestMakeLargerAndSmallerUseTheConfiguredResizeStepPort(t *testing.T) {
	settings := DefaultSettings()
	settings.ResizeStepPercent = 5
	if got, want := layoutFor(t, "make-larger", settings), (Layout{Kind: KindResizeStep, Direction: DirectionLarger, Step: 0.05}); !reflect.DeepEqual(got, want) {
		t.Fatalf("larger: layout = %+v, want %+v", got, want)
	}
	settings.ResizeStepPercent = 12
	if got, want := layoutFor(t, "make-smaller", settings), (Layout{Kind: KindResizeStep, Direction: DirectionSmaller, Step: 0.12}); !reflect.DeepEqual(got, want) {
		t.Fatalf("smaller: layout = %+v, want %+v", got, want)
	}
}

func TestGapAndSizeSettingsNeverChangeHalfMaximizeOrCenterGeometryPort(t *testing.T) {
	settings := DefaultSettings()
	settings.OuterGap = 40
	settings.InnerGap = 40
	settings.AlmostMaximizeWidthPercent = 55
	settings.ReasonableSizeWidthPercent = 35
	settings.ResizeStepPercent = 20
	if got, want := layoutFor(t, "left-half", settings), mustPreset(t, "left-half"); !reflect.DeepEqual(got, want) {
		t.Fatalf("left-half = %+v, want %+v", got, want)
	}
	if got, want := layoutFor(t, "maximize", settings), (Layout{Kind: KindFraction, X: 0, Y: 0, Width: 1, Height: 1}); !reflect.DeepEqual(got, want) {
		t.Fatalf("maximize = %+v, want %+v", got, want)
	}
	if got, want := layoutFor(t, "center", settings), (Layout{Kind: KindCenter}); !reflect.DeepEqual(got, want) {
		t.Fatalf("center = %+v, want %+v", got, want)
	}
}
