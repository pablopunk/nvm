package geometry

import "testing"

// Ported from 1.x/electron/os/window-neighbors.test.ts (29 cases) and
// 1.x/electron/os/window-neighbors-displays.test.ts (15 cases).

type neighborOverrides struct {
	zIndex       int
	displayID    string
	isFullScreen bool
}

// neighborAt builds a candidate with the TypeScript defaults: z 0, display "main", not full screen.
func neighborAt(id string, frame Rect, overrides ...func(*neighborOverrides)) NeighborWindow {
	settings := neighborOverrides{displayID: "main"}
	for _, apply := range overrides {
		apply(&settings)
	}
	return NeighborWindow{ID: id, Frame: frame, ZIndex: settings.zIndex, DisplayID: settings.displayID, IsFullScreen: settings.isFullScreen}
}

func withZ(z int) func(*neighborOverrides) {
	return func(o *neighborOverrides) { o.zIndex = z }
}

func onDisplay(id string) func(*neighborOverrides) {
	return func(o *neighborOverrides) { o.displayID = id }
}

func fullScreenWindow() func(*neighborOverrides) {
	return func(o *neighborOverrides) { o.isFullScreen = true }
}

// neighborCase is one table row. found is false when the expected answer is "no neighbor".
type neighborCase struct {
	name       string
	origin     NeighborWindow
	candidates []NeighborWindow
	direction  NeighborDirection
	expected   string
	found      bool
	options    NeighborOptions
}

func runNeighborCases(t *testing.T, cases []neighborCase) {
	t.Helper()
	for _, c := range cases {
		t.Run("nearestWindowInDirection "+c.name, func(t *testing.T) {
			got, ok := NearestWindowInDirection(c.origin, c.candidates, c.direction, c.options)
			if got != c.expected || ok != c.found {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, ok, c.expected, c.found)
			}
		})
	}
}

func concat(lists ...[]NeighborWindow) []NeighborWindow {
	var out []NeighborWindow
	for _, list := range lists {
		out = append(out, list...)
	}
	return out
}

var (
	topLeft     = neighborAt("tl", rectOf(0, 0, 100, 100))
	topRight    = neighborAt("tr", rectOf(100, 0, 100, 100))
	bottomLeft  = neighborAt("bl", rectOf(0, 100, 100, 100))
	bottomRight = neighborAt("br", rectOf(100, 100, 100, 100))
	grid        = []NeighborWindow{topLeft, topRight, bottomLeft, bottomRight}

	leftHalf  = neighborAt("left", rectOf(0, 0, 500, 900))
	rightHalf = neighborAt("right", rectOf(500, 0, 500, 900))
	halves    = []NeighborWindow{leftHalf, rightHalf}

	stackTop    = neighborAt("top", rectOf(0, 0, 800, 400))
	stackBottom = neighborAt("bottom", rectOf(0, 400, 800, 400))
	stacked     = []NeighborWindow{stackTop, stackBottom}

	cascadeFront  = neighborAt("w1", rectOf(0, 0, 600, 400))
	cascadeMiddle = neighborAt("w2", rectOf(30, 30, 600, 400), withZ(1))
	cascadeBack   = neighborAt("w3", rectOf(60, 60, 600, 400), withZ(2))

	row = []NeighborWindow{
		neighborAt("x1", rectOf(0, 0, 100, 100)),
		neighborAt("x2", rectOf(100, 0, 100, 100)),
		neighborAt("x3", rectOf(200, 0, 100, 100)),
	}

	mainWindow = neighborAt("origin", rectOf(100, 100, 800, 600))

	nearDiagonal = neighborAt("diagonal", rectOf(110, 150, 100, 100))
	farSameRow   = neighborAt("row", rectOf(500, 0, 100, 100))
	farSliver    = neighborAt("sliver", rectOf(500, 99, 100, 100))
	steepNear    = neighborAt("steep", rectOf(120, 250, 100, 100))
	flatFarther  = neighborAt("flat", rectOf(400, 100, 100, 100))
	driftedBelow = neighborAt("drifted", rectOf(1, 100, 100, 100))

	nestedOrigin  = neighborAt("big", rectOf(0, 0, 1000, 1000), withZ(1))
	cornerFront   = neighborAt("corner", rectOf(0, 0, 20, 20))
	cornerBehind  = neighborAt("corner", rectOf(0, 0, 20, 20), withZ(2))
	bigBehind     = neighborAt("big-behind", rectOf(300, 0, 1000, 1000), withZ(1))
	bigFront      = neighborAt("big-front", rectOf(300, 0, 1000, 1000))
	rowFar        = neighborAt("row-far", rectOf(1000, 100, 100, 100))
	originInFront = neighborAt("origin", rectOf(100, 100, 800, 600), withZ(1))
)

func TestNeighborGeometryCasesPort(t *testing.T) {
	runNeighborCases(t, []neighborCase{
		{name: "grid: right from top-left picks top-right", origin: topLeft, candidates: grid, direction: NeighborRight, expected: "tr", found: true},
		{name: "grid: down from top-left picks bottom-left", origin: topLeft, candidates: grid, direction: NeighborDown, expected: "bl", found: true},
		{name: "grid: left from bottom-right picks bottom-left over the diagonal", origin: bottomRight, candidates: grid, direction: NeighborLeft, expected: "bl", found: true},
		{name: "grid: up from bottom-right picks top-right", origin: bottomRight, candidates: grid, direction: NeighborUp, expected: "tr", found: true},
		{name: "grid: right from top-right has no neighbour without wrap", origin: topRight, candidates: grid, direction: NeighborRight},
		{name: "grid: right from top-right wraps to the farthest aligned window", origin: topRight, candidates: grid, direction: NeighborRight, expected: "tl", found: true, options: NeighborOptions{Wrap: true}},
		{name: "grid: up from top-left wraps to bottom-left, not bottom-right", origin: topLeft, candidates: grid, direction: NeighborUp, expected: "bl", found: true, options: NeighborOptions{Wrap: true}},
		{name: "grid: a window one point off the row does not beat the true neighbour", origin: topLeft, candidates: concat(grid, []NeighborWindow{driftedBelow}), direction: NeighborRight, expected: "tr", found: true},
		{name: "halves: right from left half selects right half", origin: leftHalf, candidates: halves, direction: NeighborRight, expected: "right", found: true},
		{name: "halves: left from left half wraps to right half", origin: leftHalf, candidates: halves, direction: NeighborLeft, expected: "right", found: true, options: NeighborOptions{Wrap: true}},
		{name: "stacked: down from top selects the window below", origin: stackTop, candidates: stacked, direction: NeighborDown, expected: "bottom", found: true},
		{name: "stacked: right ignores a window that only sits below", origin: stackTop, candidates: stacked, direction: NeighborRight},
		{name: "cascade: right from the front layer skips the layer behind that covers its centre", origin: cascadeFront, candidates: []NeighborWindow{cascadeMiddle, cascadeBack}, direction: NeighborRight},
		{name: "cascade: left from the back layer selects the nearer front layer", origin: cascadeBack, candidates: []NeighborWindow{cascadeFront, cascadeMiddle}, direction: NeighborLeft, expected: "w2", found: true},
		{name: "nested: a small window in front of a big origin is a neighbour", origin: nestedOrigin, candidates: []NeighborWindow{cornerFront}, direction: NeighborLeft, expected: "corner", found: true},
		{name: "nested: a small window hidden behind a big origin is skipped", origin: nestedOrigin, candidates: []NeighborWindow{cornerBehind}, direction: NeighborLeft},
		{name: "nested: a big window behind the origin that covers its centre is skipped", origin: mainWindow, candidates: []NeighborWindow{bigBehind, rowFar}, direction: NeighborRight, expected: "row-far", found: true},
		{name: "nested: a big window in front of the origin that covers its centre stays a candidate", origin: originInFront, candidates: []NeighborWindow{bigFront, rowFar}, direction: NeighborRight, expected: "big-front", found: true},
		{name: "diagonal: a near diagonal window beats a far window on the same row", origin: topLeft, candidates: []NeighborWindow{nearDiagonal, farSameRow}, direction: NeighborRight, expected: "diagonal", found: true},
		{name: "diagonal: a near diagonal window beats a far window with a sliver of overlap", origin: topLeft, candidates: []NeighborWindow{nearDiagonal, farSliver}, direction: NeighborRight, expected: "diagonal", found: true},
		{name: "cone: a flatter farther window beats a steep nearer one", origin: topLeft, candidates: []NeighborWindow{steepNear, flatFarther}, direction: NeighborRight, expected: "flat", found: true},
		{name: "wrap disabled returns null when nothing lies in the direction", origin: topLeft, candidates: grid, direction: NeighborLeft, options: NeighborOptions{Wrap: false}},
		{name: "wrap picks the farthest window on the same row", origin: row[2], candidates: row, direction: NeighborRight, expected: "x1", found: true, options: NeighborOptions{Wrap: true}},
		{name: "wrap is ignored when a neighbour exists in the direction", origin: row[1], candidates: row, direction: NeighborRight, expected: "x3", found: true, options: NeighborOptions{Wrap: true}},
		{name: "lone window has no neighbour in any direction", origin: topLeft, candidates: []NeighborWindow{topLeft}, direction: NeighborRight, options: NeighborOptions{Wrap: true}},
		{name: "zero-area candidates are ignored", origin: topLeft, candidates: []NeighborWindow{neighborAt("sliver", rectOf(150, 0, 0, 100))}, direction: NeighborRight},
		{name: "zero-area origin has no neighbour", origin: neighborAt("flat", rectOf(0, 0, 0, 100)), candidates: []NeighborWindow{topRight}, direction: NeighborRight},
		{name: "equal scores tie-break on the front-most window before the smaller id", origin: topLeft, candidates: []NeighborWindow{neighborAt("a", rectOf(100, 0, 100, 100), withZ(1)), neighborAt("b", rectOf(100, 0, 100, 100), withZ(0))}, direction: NeighborRight, expected: "b", found: true},
		{name: "equal scores with equal z tie-break on the smaller id", origin: topLeft, candidates: []NeighborWindow{neighborAt("z", rectOf(100, 0, 100, 100)), neighborAt("b", rectOf(100, 0, 100, 100))}, direction: NeighborRight, expected: "b", found: true},
	})
}

var (
	mainLeft        = neighborAt("main-left", rectOf(0, 0, 50, 50))
	leftNear        = neighborAt("left-near", rectOf(-200, 380, 100, 40), onDisplay("left"))
	leftDisplayWin  = neighborAt("left-display", rectOf(-1500, 200, 1000, 600), onDisplay("left"))
	rightDisplayWin = neighborAt("right-display", rectOf(1500, 100, 1000, 600), onDisplay("right"))
	aboveWindow     = neighborAt("above", rectOf(100, -900, 800, 600), onDisplay("above"))
	fullScreenOrig  = neighborAt("full", rectOf(0, 0, 1440, 900), fullScreenWindow())
	sameDisplaySide = neighborAt("same-side", rectOf(900, 0, 500, 900), withZ(1))
	otherDisplay    = neighborAt("other", rectOf(1440, 0, 1440, 900), onDisplay("right"))
	stackMiddle     = neighborAt("mid", rectOf(0, 0, 100, 100), withZ(1))
	stackFront      = neighborAt("front", rectOf(0, 0, 100, 100))
	stackBack       = neighborAt("back", rectOf(0, 0, 100, 100), withZ(2))
	stackFarBack    = neighborAt("far-back", rectOf(0, 0, 100, 100), withZ(3))
)

func TestNeighborDisplayCasesPort(t *testing.T) {
	runNeighborCases(t, []neighborCase{
		{name: "second display: left reaches a window on the display to the left", origin: mainWindow, candidates: []NeighborWindow{leftDisplayWin}, direction: NeighborLeft, expected: "left-display", found: true},
		{name: "second display: the origin display is preferred over a closer window on another display", origin: mainWindow, candidates: []NeighborWindow{leftNear, mainLeft}, direction: NeighborLeft, expected: "main-left", found: true},
		{name: "second display: up reaches the window on the display above", origin: mainWindow, candidates: []NeighborWindow{leftDisplayWin, aboveWindow}, direction: NeighborUp, expected: "above", found: true},
		{name: "second display: down from the display above returns to the origin display", origin: aboveWindow, candidates: []NeighborWindow{mainWindow}, direction: NeighborDown, expected: "origin", found: true},
		{name: "second display: a window on the right is not reached by left without wrap", origin: mainWindow, candidates: []NeighborWindow{rightDisplayWin}, direction: NeighborLeft},
		{name: "second display: wrap reaches a window on the opposite display", origin: mainWindow, candidates: []NeighborWindow{rightDisplayWin}, direction: NeighborLeft, expected: "right-display", found: true, options: NeighborOptions{Wrap: true}},
		{name: "full-screen origin ignores windows on its own display", origin: fullScreenOrig, candidates: []NeighborWindow{sameDisplaySide}, direction: NeighborRight},
		{name: "full-screen origin reaches a window on another display", origin: fullScreenOrig, candidates: []NeighborWindow{sameDisplaySide, otherDisplay}, direction: NeighborRight, expected: "other", found: true},
		{name: "full-screen origin wrap still ignores windows on its own display", origin: fullScreenOrig, candidates: []NeighborWindow{neighborAt("same-left", rectOf(0, 0, 200, 900), withZ(1))}, direction: NeighborLeft, options: NeighborOptions{Wrap: true}},
		{name: "stacked: right from the middle of a stack selects the next window behind", origin: stackMiddle, candidates: []NeighborWindow{stackFront, stackBack, stackFarBack}, direction: NeighborRight, expected: "back", found: true},
		{name: "stacked: left from the middle of a stack selects the window in front", origin: stackMiddle, candidates: []NeighborWindow{stackFront, stackBack, stackFarBack}, direction: NeighborLeft, expected: "front", found: true},
		{name: "stacked: a coincident window beats a farther geometric neighbour", origin: stackMiddle, candidates: []NeighborWindow{stackFront, stackBack, farSameRow}, direction: NeighborRight, expected: "back", found: true},
		{name: "stacked: a window whose centre is within two points counts as stacked", origin: topLeft, candidates: []NeighborWindow{neighborAt("near-stack", rectOf(1, 1, 100, 100), withZ(1)), farSameRow}, direction: NeighborRight, expected: "near-stack", found: true},
		{name: "stacked: a coincident stack elsewhere is entered at its front-most window", origin: topLeft, candidates: []NeighborWindow{neighborAt("tr-back", rectOf(100, 0, 100, 100), withZ(2)), neighborAt("tr-front", rectOf(100, 0, 100, 100), withZ(1))}, direction: NeighborRight, expected: "tr-front", found: true},
		{name: "order: the answer does not depend on candidate order", origin: topLeft, candidates: []NeighborWindow{bottomRight, bottomLeft, topRight}, direction: NeighborRight, expected: "tr", found: true},
	})
}
