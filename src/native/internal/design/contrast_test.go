package design

import (
	"math"
	"testing"
)

var (
	white = Color{255, 255, 255, 255}
	black = Color{0, 0, 0, 255}
)

func gray(v uint8) Color { return Color{v, v, v, 255} }

// TestContrastMatchesKnownWCAGRatios checks the ratios that WCAG and common palettes publish.
func TestContrastMatchesKnownWCAGRatios(t *testing.T) {
	tests := []struct {
		name string
		fg   Color
		bg   Color
		want float64
	}{
		{name: "black on white", fg: black, bg: white, want: 21},
		{name: "white on black", fg: white, bg: black, want: 21},
		{name: "same color", fg: gray(128), bg: gray(128), want: 1},
		{name: "#595959 on white is 7:1", fg: gray(0x59), bg: white, want: 7.00},
		{name: "#767676 on white is the 4.5:1 gray", fg: gray(0x76), bg: white, want: 4.54},
		{name: "#777777 on white just misses 4.5:1", fg: gray(0x77), bg: white, want: 4.48},
		{name: "#949494 on white is the 3:1 gray", fg: gray(0x94), bg: white, want: 3.03},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Contrast(tt.fg, tt.bg)
			if math.Abs(got-tt.want) > 0.01 {
				t.Fatalf("Contrast = %.3f, want %.2f", got, tt.want)
			}
		})
	}
}

func TestLuminanceOfPrimaries(t *testing.T) {
	tests := []struct {
		name string
		c    Color
		want float64
	}{
		{name: "white", c: white, want: 1},
		{name: "black", c: black, want: 0},
		{name: "#808080 is 0.2159", c: gray(128), want: 0.2159},
		{name: "pure green is 0.7152", c: Color{0, 255, 0, 255}, want: 0.7152},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Luminance(tt.c); math.Abs(got-tt.want) > 0.0005 {
				t.Fatalf("Luminance = %.4f, want %.4f", got, tt.want)
			}
		})
	}
}

func TestOverCompositesAlphaOverBackdrop(t *testing.T) {
	half := Color{255, 255, 255, 128}
	if got := Over(half, black); got.R < 127 || got.R > 128 || got.A != 255 {
		t.Fatalf("white at half alpha over black = %+v, want about #808080 opaque", got)
	}
	if got := Over(black, white); got != black {
		t.Fatalf("opaque black over white = %+v, want black", got)
	}
	if got := Over(Color{}, white); got != white {
		t.Fatalf("transparent over white = %+v, want white", got)
	}
}

func TestTransparentTextHasNoContrast(t *testing.T) {
	if got := Contrast(Color{0, 0, 0, 0}, white); math.Abs(got-1) > 0.001 {
		t.Fatalf("fully transparent text contrast = %.3f, want 1", got)
	}
}

func TestContrastRejectsTranslucentBackground(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Contrast on a translucent background did not panic")
		}
	}()
	Contrast(black, Color{255, 255, 255, 128})
}

func TestAAThresholds(t *testing.T) {
	tests := []struct {
		name  string
		fg    Color
		large bool
		want  bool
	}{
		{name: "#767676 passes normal text", fg: gray(0x76), large: false, want: true},
		{name: "#777777 fails normal text", fg: gray(0x77), large: false, want: false},
		{name: "#777777 passes large text", fg: gray(0x77), large: true, want: true},
		{name: "#949494 just passes large text", fg: gray(0x94), large: true, want: true},
		{name: "#959595 fails even large text", fg: gray(0x95), large: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AA(tt.fg, white, tt.large); got != tt.want {
				t.Fatalf("AA = %v (ratio %.3f), want %v", got, Contrast(tt.fg, white), tt.want)
			}
		})
	}
}
