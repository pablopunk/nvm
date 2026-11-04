package windowmgmt

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

func TestOnlyFractionLayoutsReachExtensions(t *testing.T) {
	fraction := &api.Rect{X: 0, Y: 0, Width: 0.5, Height: 1}
	layout, ok := fractionLayoutOf(api.WindowLayout{Kind: api.LayoutFraction, Fraction: fraction})
	if !ok || layout.Kind != geometry.KindFraction || layout.Width != 0.5 || layout.Height != 1 {
		t.Fatalf("fraction layout = %+v (%t)", layout, ok)
	}
	if _, ok := fractionLayoutOf(api.WindowLayout{Kind: api.LayoutCenter}); ok {
		t.Fatal("a center layout reached extensions")
	}
	if _, ok := fractionLayoutOf(api.WindowLayout{Kind: api.LayoutFraction}); ok {
		t.Fatal("a fraction layout without its rectangle was accepted")
	}
}

func TestExtensionOptionsStartFromThePaletteDefaults(t *testing.T) {
	gap := 8
	options := geometryOptionsOf(api.WindowManagementOptions{InnerGap: &gap})
	defaults := geometry.DefaultOptions()
	if options.InnerGap != 8 || options.OuterGap != defaults.OuterGap || options.Cycle != defaults.Cycle {
		t.Fatalf("options = %+v, want the defaults with the inner gap set", options)
	}
}

func TestOutcomeNamesMatchTheAPI(t *testing.T) {
	cases := map[geometry.Outcome]string{geometry.Applied: "applied", geometry.Partial: "partial", geometry.Failed: "failed"}
	for outcome, want := range cases {
		if got := outcomeName(outcome); got != want {
			t.Errorf("outcomeName(%v) = %q, want %q", outcome, got, want)
		}
	}
}
