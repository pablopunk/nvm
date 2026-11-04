package geometry

import (
	"fmt"
	"testing"
)

// Ported from src/app/electron/os/window-layout-gaps.test.ts.

var (
	gapsWorkArea     = rectOf(0, 0, 1440, 900)
	gapsCurrentFrame = rectOf(100, 120, 800, 500)
	outer8           = EdgeGaps{Top: 8, Right: 8, Bottom: 8, Left: 8}
	perEdgeOuter     = EdgeGaps{Top: 10, Right: 20, Bottom: 30, Left: 40}
)

func gapsWith(outer EdgeGaps, inner float64, applyToMaximize bool) Options {
	options := DefaultOptions()
	options.OuterGap = outer
	options.InnerGap = inner
	options.ApplyGapToMaximize = applyToMaximize
	return options
}

func gapFrameFor(t *testing.T, layout Layout, options Options, workArea Rect) Rect {
	t.Helper()
	return targetFrame(t, layout, workArea, options, gapsCurrentFrame)
}

func gapLayoutFrame(t *testing.T, id PresetID, options Options, workArea Rect) Rect {
	t.Helper()
	return gapFrameFor(t, mustPreset(t, id), options, workArea)
}

func TestGapsPort(t *testing.T) {
	t.Run("outer 8 and inner 4 leave 8px at the display edges and exactly 4px between halves", func(t *testing.T) {
		options := gapsWith(outer8, 4, true)
		assertFrame(t, gapLayoutFrame(t, "left-half", options, gapsWorkArea), rectOf(8, 8, 710, 884))
		assertFrame(t, gapLayoutFrame(t, "right-half", options, gapsWorkArea), rectOf(722, 8, 710, 884))
	})

	t.Run("per-edge outer gaps inset each display edge by its own value", func(t *testing.T) {
		options := gapsWith(perEdgeOuter, 6, true)
		assertFrame(t, gapLayoutFrame(t, "left-half", options, gapsWorkArea), rectOf(40, 10, 677, 860))
		assertFrame(t, gapLayoutFrame(t, "right-half", options, gapsWorkArea), rectOf(723, 10, 697, 860))
	})

	t.Run("quarters split the inner gap on both axes and keep outer gaps on the display corners", func(t *testing.T) {
		options := gapsWith(outer8, 4, true)
		assertFrame(t, gapLayoutFrame(t, "top-left-quarter", options, gapsWorkArea), rectOf(8, 8, 710, 440))
		assertFrame(t, gapLayoutFrame(t, "bottom-right-quarter", options, gapsWorkArea), rectOf(722, 452, 710, 440))
	})

	t.Run("first and last thirds keep the inner gap to the center third", func(t *testing.T) {
		options := gapsWith(outer8, 4, true)
		first := gapLayoutFrame(t, "first-third", options, gapsWorkArea)
		center := gapLayoutFrame(t, "center-third", options, gapsWorkArea)
		last := gapLayoutFrame(t, "last-third", options, gapsWorkArea)
		assertFrame(t, first, rectOf(8, 8, 470, 884))
		assertFrame(t, last, rectOf(962, 8, 470, 884))
		if first.X+first.Width+4 != center.X {
			t.Fatalf("first to center gap = %v, want 4", center.X-(first.X+first.Width))
		}
		if center.X+center.Width+4 != last.X {
			t.Fatalf("center to last gap = %v, want 4", last.X-(center.X+center.Width))
		}
	})

	t.Run("top and bottom halves use the vertical outer gaps and the inner gap between them", func(t *testing.T) {
		options := gapsWith(perEdgeOuter, 6, true)
		assertFrame(t, gapLayoutFrame(t, "top-half", options, gapsWorkArea), rectOf(40, 10, 1380, 437))
		assertFrame(t, gapLayoutFrame(t, "bottom-half", options, gapsWorkArea), rectOf(40, 453, 1380, 417))
	})

	t.Run("two-thirds tiles touch the outer gap on their display edge only", func(t *testing.T) {
		options := gapsWith(outer8, 4, true)
		assertFrame(t, gapLayoutFrame(t, "top-two-thirds", options, gapsWorkArea), rectOf(8, 8, 1424, 590))
	})

	t.Run("maximize applies every outer gap when applyToMaximize is true", func(t *testing.T) {
		assertFrame(t, gapLayoutFrame(t, "maximize", gapsWith(perEdgeOuter, 6, true), gapsWorkArea), rectOf(40, 10, 1380, 860))
	})

	t.Run("maximize has no gaps when applyToMaximize is false", func(t *testing.T) {
		assertFrame(t, gapLayoutFrame(t, "maximize", gapsWith(outer8, 4, false), gapsWorkArea), rectOf(0, 0, 1440, 900))
	})

	t.Run("halves keep their outer gaps when applyToMaximize is false", func(t *testing.T) {
		assertFrame(t, gapLayoutFrame(t, "left-half", gapsWith(outer8, 4, false), gapsWorkArea), rectOf(8, 8, 710, 884))
	})

	t.Run("maximize-width uses the left and right outer gaps and keeps vertical position", func(t *testing.T) {
		frame := gapFrameFor(t, Layout{Kind: KindMaximizeWidth}, gapsWith(perEdgeOuter, 6, true), gapsWorkArea)
		assertFrame(t, frame, rectOf(40, 120, 1380, 500))
	})

	t.Run("maximize-width has no horizontal gap when applyToMaximize is false", func(t *testing.T) {
		frame := gapFrameFor(t, Layout{Kind: KindMaximizeWidth}, gapsWith(outer8, 4, false), gapsWorkArea)
		assertFrame(t, frame, rectOf(0, 120, 1440, 500))
	})

	t.Run("maximize-height uses the top and bottom outer gaps and keeps horizontal position", func(t *testing.T) {
		frame := gapFrameFor(t, Layout{Kind: KindMaximizeHeight}, gapsWith(perEdgeOuter, 6, true), gapsWorkArea)
		assertFrame(t, frame, rectOf(100, 10, 800, 860))
	})

	t.Run("maximize-height has no vertical gap when applyToMaximize is false", func(t *testing.T) {
		frame := gapFrameFor(t, Layout{Kind: KindMaximizeHeight}, gapsWith(outer8, 4, false), gapsWorkArea)
		assertFrame(t, frame, rectOf(100, 0, 800, 900))
	})

	t.Run("edge moves use the outer gap of the edge they move to", func(t *testing.T) {
		options := gapsWith(perEdgeOuter, 6, true)
		assertFrame(t, gapFrameFor(t, Layout{Kind: KindEdge, Edge: EdgeLeft}, options, gapsWorkArea), rectOf(40, 120, 800, 500))
		assertFrame(t, gapFrameFor(t, Layout{Kind: KindEdge, Edge: EdgeRight}, options, gapsWorkArea), rectOf(620, 120, 800, 500))
		assertFrame(t, gapFrameFor(t, Layout{Kind: KindEdge, Edge: EdgeTop}, options, gapsWorkArea), rectOf(100, 10, 800, 500))
		assertFrame(t, gapFrameFor(t, Layout{Kind: KindEdge, Edge: EdgeBottom}, options, gapsWorkArea), rectOf(100, 370, 800, 500))
	})

	t.Run("gaps larger than a tiny work area clamp the tile to zero width inside the area", func(t *testing.T) {
		tiny := rectOf(0, 0, 20, 20)
		frame := gapLayoutFrame(t, "left-half", UniformGaps(40), tiny)
		if frame.Width != 0 {
			t.Fatalf("width = %v, want 0", frame.Width)
		}
		if !(frame.X >= tiny.X && frame.X <= tiny.X+tiny.Width) {
			t.Fatalf("x = %v outside the tiny area", frame.X)
		}
		if !(frame.Y >= tiny.Y && frame.Y <= tiny.Y+tiny.Height) {
			t.Fatalf("y = %v outside the tiny area", frame.Y)
		}
	})

	t.Run("per-edge gaps on a negative-origin display keep the outer gaps on the display edges", func(t *testing.T) {
		negativeArea := rectOf(-1920, 0, 1920, 1080)
		options := gapsWith(perEdgeOuter, 6, true)
		assertFrame(t, gapLayoutFrame(t, "right-half", options, negativeArea), rectOf(-957, 10, 937, 1040))
		if got := gapLayoutFrame(t, "left-half", options, negativeArea).X; got != negativeArea.X+perEdgeOuter.Left {
			t.Fatalf("left-half x = %v, want %v", got, negativeArea.X+perEdgeOuter.Left)
		}
	})

	t.Run("zero gaps reproduce the abutting halves of the plain work area", func(t *testing.T) {
		assertFrame(t, gapLayoutFrame(t, "left-half", UniformGaps(0), gapsWorkArea), rectOf(0, 0, 720, 900))
	})
}

func TestOddWidthHalvesWithGapsPort(t *testing.T) {
	oddWorkArea := rectOf(0, 0, 1441, 900)
	outers := []EdgeGaps{
		{Top: 0, Right: 0, Bottom: 0, Left: 0},
		{Top: 8, Right: 8, Bottom: 8, Left: 8},
		{Top: 3, Right: 7, Bottom: 5, Left: 9},
	}
	inners := []float64{0, 1, 4, 5, 11}
	for _, outer := range outers {
		for _, inner := range inners {
			name := fmt.Sprintf("odd-width halves with outer %v/%v and inner %v keep exactly %vpx between them", outer.Left, outer.Right, inner, inner)
			t.Run(name, func(t *testing.T) {
				options := gapsWith(outer, inner, true)
				left := gapLayoutFrame(t, "left-half", options, oddWorkArea)
				right := gapLayoutFrame(t, "right-half", options, oddWorkArea)
				if left.X != outer.Left {
					t.Fatalf("left.X = %v, want %v", left.X, outer.Left)
				}
				if right.X+right.Width != oddWorkArea.Width-outer.Right {
					t.Fatalf("right edge = %v, want %v", right.X+right.Width, oddWorkArea.Width-outer.Right)
				}
				if left.X+left.Width+inner != right.X {
					t.Fatalf("gap = %v, want %v", right.X-(left.X+left.Width), inner)
				}
			})
		}
	}
}
