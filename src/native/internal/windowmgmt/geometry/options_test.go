package geometry

import (
	"reflect"
	"testing"
)

// Ported from src/app/electron/os/window-management-options.test.ts and the defaults-to-options
// cases of src/app/extensions/window-management-options.test.ts. In Go the partial-options
// resolution is expressed by starting from DefaultOptions() and overriding fields; TypeScript
// undefined fields have no Go equivalent (see the report).

func TestOptionsPartialOverridesPort(t *testing.T) {
	t.Run("no partial options resolves to the defaults", func(t *testing.T) {
		want := Options{
			OuterGap:           EdgeGaps{Top: 8, Right: 8, Bottom: 8, Left: 8},
			InnerGap:           4,
			ApplyGapToMaximize: true,
			Cycle:              true,
			CycleOrder:         CycleHalfTwoThirdsThird,
			DisplayOrder:       DisplayOrderLeftToRight,
			DisplayMoveSizing:  MoveProportional,
		}
		if got := DefaultOptions(); !reflect.DeepEqual(got, want) {
			t.Fatalf("DefaultOptions() = %+v, want %+v", got, want)
		}
	})

	t.Run("a partial outer gap overrides only the edges it names", func(t *testing.T) {
		options := DefaultOptions()
		options.OuterGap.Right = 20
		options.OuterGap.Bottom = 0
		if want := (EdgeGaps{Top: 8, Right: 20, Bottom: 0, Left: 8}); options.OuterGap != want {
			t.Fatalf("outer gap = %+v, want %+v", options.OuterGap, want)
		}
		if options.InnerGap != DefaultOptions().InnerGap {
			t.Fatalf("inner gap changed to %v", options.InnerGap)
		}
	})

	t.Run("scalar options override their defaults", func(t *testing.T) {
		options := DefaultOptions()
		options.InnerGap = 12
		options.ApplyGapToMaximize = false
		options.Cycle = false
		options.CycleOrder = CycleHalfThirdTwoThirds
		options.DisplayOrder = DisplayOrderTopToBottom
		options.DisplayMoveSizing = MoveKeepSize
		want := Options{
			OuterGap:           DefaultOptions().OuterGap,
			InnerGap:           12,
			ApplyGapToMaximize: false,
			Cycle:              false,
			CycleOrder:         CycleHalfThirdTwoThirds,
			DisplayOrder:       DisplayOrderTopToBottom,
			DisplayMoveSizing:  MoveKeepSize,
		}
		if !reflect.DeepEqual(options, want) {
			t.Fatalf("options = %+v, want %+v", options, want)
		}
	})

	t.Run("resolving never mutates the shared defaults or the partial input", func(t *testing.T) {
		options := DefaultOptions()
		options.OuterGap.Top = 99
		if got := DefaultOptions().OuterGap.Top; got != 8 {
			t.Fatalf("defaults mutated: top = %v", got)
		}
	})

	t.Run("the engine gap fields are carried by Options unchanged", func(t *testing.T) {
		options := DefaultOptions()
		options.OuterGap = EdgeGaps{Top: 1, Right: 2, Bottom: 3, Left: 4}
		options.InnerGap = 5
		options.ApplyGapToMaximize = false
		if options.OuterGap != (EdgeGaps{Top: 1, Right: 2, Bottom: 3, Left: 4}) || options.InnerGap != 5 || options.ApplyGapToMaximize {
			t.Fatalf("gaps not carried: %+v", options)
		}
	})
}

func TestOptionsFromSettingsPort(t *testing.T) {
	withSettings := func(mutate func(*Settings)) Options {
		settings := DefaultSettings()
		mutate(&settings)
		return OptionsFromSettings(settings)
	}

	t.Run("default settings build the host default options", func(t *testing.T) {
		if got := OptionsFromSettings(DefaultSettings()); !reflect.DeepEqual(got, DefaultOptions()) {
			t.Fatalf("options = %+v, want %+v", got, DefaultOptions())
		}
	})

	t.Run("the outer gap fills every edge that has no override", func(t *testing.T) {
		options := withSettings(func(s *Settings) { s.OuterGap = 12 })
		if want := (EdgeGaps{Top: 12, Right: 12, Bottom: 12, Left: 12}); options.OuterGap != want {
			t.Fatalf("outer gap = %+v, want %+v", options.OuterGap, want)
		}
	})

	t.Run("per-edge overrides win over the outer gap and leave other edges alone", func(t *testing.T) {
		options := withSettings(func(s *Settings) {
			s.OuterGap = 10
			s.OuterGapTop = floatPtr(0)
			s.OuterGapLeft = floatPtr(24)
		})
		if want := (EdgeGaps{Top: 0, Right: 10, Bottom: 10, Left: 24}); options.OuterGap != want {
			t.Fatalf("outer gap = %+v, want %+v", options.OuterGap, want)
		}
	})

	t.Run("a zero override is a real override, not a missing one", func(t *testing.T) {
		options := withSettings(func(s *Settings) {
			s.OuterGap = 8
			s.OuterGapBottom = floatPtr(0)
		})
		if options.OuterGap.Bottom != 0 {
			t.Fatalf("bottom = %v, want 0", options.OuterGap.Bottom)
		}
	})

	t.Run("gap, cycle, and display options pass through unchanged", func(t *testing.T) {
		options := withSettings(func(s *Settings) {
			s.InnerGap = 14
			s.ApplyGapToMaximize = false
			s.Cycle = false
			s.CycleOrder = CycleHalfThirdTwoThirds
			s.DisplayOrder = DisplayOrderTopToBottom
			s.DisplayMoveSizing = MoveKeepSize
		})
		want := Options{
			OuterGap:           EdgeGaps{Top: 8, Right: 8, Bottom: 8, Left: 8},
			InnerGap:           14,
			ApplyGapToMaximize: false,
			Cycle:              false,
			CycleOrder:         CycleHalfThirdTwoThirds,
			DisplayOrder:       DisplayOrderTopToBottom,
			DisplayMoveSizing:  MoveKeepSize,
		}
		if !reflect.DeepEqual(options, want) {
			t.Fatalf("options = %+v, want %+v", options, want)
		}
	})
}

func TestUniformGapsMatchesTheFixture(t *testing.T) {
	options := UniformGaps(10)
	if want := (EdgeGaps{Top: 10, Right: 10, Bottom: 10, Left: 10}); options.OuterGap != want || options.InnerGap != 10 || !options.ApplyGapToMaximize {
		t.Fatalf("UniformGaps(10) = %+v", options)
	}
}
