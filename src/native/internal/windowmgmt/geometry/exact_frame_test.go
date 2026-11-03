package geometry

import "testing"

// Ported from src/app/electron/os/window-layout-exact.test.ts.

var (
	menuBarWorkArea        = rectOf(0, 25, 1440, 875)
	exactCurrentFrame      = rectOf(100, 120, 800, 500)
	negativeOriginWorkArea = rectOf(-1920, 0, 1920, 1080)
	abovePrimaryWorkArea   = rectOf(0, -1080, 1920, 1080)
	portraitWorkArea       = rectOf(0, 0, 1080, 1920)
	oddExactWorkArea       = rectOf(0, 25, 1441, 875)
)

func pointsDim(value float64) *Dimension {
	return &Dimension{Value: value, Unit: UnitPoints}
}

func percentDim(value float64) *Dimension {
	return &Dimension{Value: value, Unit: UnitPercent}
}

type exactFrameCase struct {
	name     string
	layout   Layout
	expected Rect
	workArea *Rect
	current  *Rect
	gap      float64
}

func TestExactFrameCasesPort(t *testing.T) {
	cases := []exactFrameCase{
		{
			name:     "absolute points place and size the window",
			layout:   exactLayout(pointsDim(100), pointsDim(50), pointsDim(400), pointsDim(300)),
			expected: rectOf(100, 75, 400, 300),
		},
		{
			name:     "percent sizes scale to the work area and round to whole points",
			layout:   exactLayout(nil, nil, percentDim(50), percentDim(50)),
			expected: rectOf(100, 120, 720, 438),
		},
		{
			name:     "percent offsets are measured from the work area origin",
			layout:   exactLayout(percentDim(10), percentDim(20), nil, nil),
			expected: rectOf(144, 200, 800, 500),
		},
		{
			name:     "only x and y move the window and keep its size",
			layout:   exactLayout(pointsDim(0), pointsDim(0), nil, nil),
			expected: rectOf(0, 25, 800, 500),
		},
		{
			name:     "only width and height resize the window and keep its position",
			layout:   exactLayout(nil, nil, pointsDim(1000), pointsDim(300)),
			expected: rectOf(100, 120, 1000, 300),
		},
		{
			name:     "a partial x and width keeps the current height and y",
			layout:   exactLayout(pointsDim(500), nil, pointsDim(300), nil),
			expected: rectOf(500, 120, 300, 500),
		},
		{
			name:     "a position past the right edge clamps the window inside the work area",
			layout:   exactLayout(pointsDim(1300), nil, pointsDim(400), pointsDim(300)),
			expected: rectOf(1040, 120, 400, 300),
		},
		{
			name:     "a kept position below the work area clamps the window back inside",
			layout:   exactLayout(nil, nil, pointsDim(400), nil),
			current:  rectPtr(rectOf(100, 2000, 800, 500)),
			expected: rectOf(100, 400, 400, 500),
		},
		{
			name:     "a width larger than the work area shrinks to fit before clamping",
			layout:   exactLayout(nil, nil, pointsDim(2000), nil),
			expected: rectOf(0, 120, 1440, 500),
		},
		{
			name:     "a height larger than the work area shrinks to fit and pins to the top",
			layout:   exactLayout(nil, nil, nil, pointsDim(5000)),
			expected: rectOf(100, 25, 800, 875),
		},
		{
			name:     "a negative-origin work area places offsets from its own origin",
			layout:   exactLayout(percentDim(10), pointsDim(0), percentDim(50), percentDim(100)),
			workArea: rectPtr(negativeOriginWorkArea),
			expected: rectOf(-1728, 0, 960, 1080),
		},
		{
			name:     "an above-primary work area places offsets from its own origin",
			layout:   exactLayout(nil, pointsDim(0), nil, percentDim(50)),
			workArea: rectPtr(abovePrimaryWorkArea),
			expected: rectOf(100, -1080, 800, 540),
		},
		{
			name:     "a portrait work area scales each axis by its own extent",
			layout:   exactLayout(nil, nil, percentDim(100), percentDim(50)),
			workArea: rectPtr(portraitWorkArea),
			expected: rectOf(0, 120, 1080, 960),
		},
		{
			name:     "odd work area sizes round each resolved dimension",
			layout:   exactLayout(nil, nil, percentDim(33), percentDim(33.3)),
			workArea: rectPtr(oddExactWorkArea),
			expected: rectOf(100, 120, 476, 291),
		},
		{
			name:     "a sub-point percent size rounds up to one point",
			layout:   exactLayout(nil, nil, percentDim(0.001), nil),
			expected: rectOf(100, 120, 1, 500),
		},
		{
			name:     "fractional point offsets round to whole points",
			layout:   exactLayout(pointsDim(10.4), pointsDim(10.6), nil, nil),
			expected: rectOf(10, 36, 800, 500),
		},
		{
			name:     "the gap does not inset an exact frame",
			layout:   exactLayout(pointsDim(0), pointsDim(0), pointsDim(720), pointsDim(875)),
			gap:      40,
			expected: rectOf(0, 25, 720, 875),
		},
	}
	for _, c := range cases {
		t.Run("exact layout: "+c.name, func(t *testing.T) {
			workArea := menuBarWorkArea
			if c.workArea != nil {
				workArea = *c.workArea
			}
			current := exactCurrentFrame
			if c.current != nil {
				current = *c.current
			}
			frame := targetFrame(t, c.layout, workArea, UniformGaps(c.gap), current)
			assertFrame(t, frame, c.expected)
		})
	}
}

func TestExactLayoutWithSameWorkAreaAndFrameKeepsTheFrame(t *testing.T) {
	layout := exactLayout(pointsDim(0), pointsDim(0), pointsDim(800), pointsDim(500))
	frame := targetFrame(t, layout, menuBarWorkArea, UniformGaps(0), rectOf(0, 25, 800, 500))
	assertFrame(t, frame, rectOf(0, 25, 800, 500))
}

func rectPtr(rect Rect) *Rect {
	return &rect
}
