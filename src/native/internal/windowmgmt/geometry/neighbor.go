package geometry

import "math"

// NeighborDirection is the direction a focus-neighbor command moves in.
type NeighborDirection string

const (
	NeighborLeft  NeighborDirection = "left"
	NeighborRight NeighborDirection = "right"
	NeighborUp    NeighborDirection = "up"
	NeighborDown  NeighborDirection = "down"
)

// NeighborOptions control whether a search without a neighbor wraps to the far side.
type NeighborOptions struct {
	Wrap bool
}

// NeighborWindow is one candidate window. ZIndex is 0 for the front-most window; larger is further
// back. IDs must be unique among candidates; they are the final tie-break.
type NeighborWindow struct {
	ID           string
	Frame        Rect
	ZIndex       int
	DisplayID    string
	IsFullScreen bool
}

const (
	coincidentCenterTolerance = 2
	hiddenCoverageRatio       = 0.95
)

// axisKind names the screen axis a direction moves along.
type axisKind int

const (
	axisX axisKind = iota
	axisY
)

// directionAxis gives the axis and sign of each direction (DIRECTION_AXES).
var directionAxis = map[NeighborDirection]struct {
	axis axisKind
	sign float64
}{
	NeighborRight: {axis: axisX, sign: 1},
	NeighborLeft:  {axis: axisX, sign: -1},
	NeighborDown:  {axis: axisY, sign: 1},
	NeighborUp:    {axis: axisY, sign: -1},
}

func coordinateOn(point Point, axis axisKind) float64 {
	if axis == axisX {
		return point.X
	}
	return point.Y
}

func perpendicularTo(axis axisKind) axisKind {
	if axis == axisX {
		return axisY
	}
	return axisX
}

func containsPoint(frame Rect, point Point) bool {
	return point.X >= frame.X && point.X <= frame.X+frame.Width &&
		point.Y >= frame.Y && point.Y <= frame.Y+frame.Height
}

// coveredRatio is the share of covered's area that cover overlaps.
func coveredRatio(covered, cover Rect) float64 {
	width := math.Min(covered.X+covered.Width, cover.X+cover.Width) - math.Max(covered.X, cover.X)
	height := math.Min(covered.Y+covered.Height, cover.Y+cover.Height) - math.Max(covered.Y, cover.Y)
	return (math.Max(0, width) * math.Max(0, height)) / (covered.Width * covered.Height)
}

// isHiddenBehindOrigin reports whether a window behind the origin covers its center, so it cannot be
// the neighbor a user sees.
func isHiddenBehindOrigin(origin, candidate NeighborWindow) bool {
	return candidate.ZIndex > origin.ZIndex &&
		(containsPoint(candidate.Frame, origin.Frame.Center()) ||
			coveredRatio(candidate.Frame, origin.Frame) >= hiddenCoverageRatio)
}

// coneWeightedDistance scores a candidate: axial distance plus a penalty that grows with the
// sideways offset, so a flatter, farther window can beat a steep, nearer one.
func coneWeightedDistance(axialDistance, perpendicularOffset float64) float64 {
	return (axialDistance*axialDistance + perpendicularOffset*perpendicularOffset) / axialDistance
}

// measurement is one candidate's standing for a direction search.
type measurement struct {
	id                  string
	ahead               bool
	displayRank         int
	score               float64
	stackStep           float64
	axialDistance       float64
	perpendicularOffset float64
	zIndex              int
}

func displayRankOf(origin, candidate NeighborWindow) int {
	if candidate.DisplayID == origin.DisplayID {
		return 0
	}
	return 1
}

// stackedMeasurement handles a candidate whose center coincides with the origin's (a stack). It
// counts only when it sits further back in the stack in the requested direction.
func stackedMeasurement(origin, candidate NeighborWindow, direction NeighborDirection, displayRank int) *measurement {
	stackStep := directionAxis[direction].sign * float64(candidate.ZIndex-origin.ZIndex)
	if stackStep <= 0 {
		return nil
	}
	return &measurement{id: candidate.ID, ahead: true, displayRank: displayRank, stackStep: stackStep, zIndex: candidate.ZIndex}
}

func geometricMeasurement(origin, candidate NeighborWindow, direction NeighborDirection, displayRank int) *measurement {
	if isHiddenBehindOrigin(origin, candidate) {
		return nil
	}
	spec := directionAxis[direction]
	originCenter := origin.Frame.Center()
	candidateCenter := candidate.Frame.Center()
	signedAxial := spec.sign * (coordinateOn(candidateCenter, spec.axis) - coordinateOn(originCenter, spec.axis))
	if signedAxial == 0 {
		return nil
	}
	perpendicular := perpendicularTo(spec.axis)
	axialDistance := math.Abs(signedAxial)
	perpendicularOffset := math.Abs(coordinateOn(candidateCenter, perpendicular) - coordinateOn(originCenter, perpendicular))
	return &measurement{
		id:                  candidate.ID,
		ahead:               signedAxial > 0,
		displayRank:         displayRank,
		score:               coneWeightedDistance(axialDistance, perpendicularOffset),
		axialDistance:       axialDistance,
		perpendicularOffset: perpendicularOffset,
		zIndex:              candidate.ZIndex,
	}
}

func measureNeighbor(origin, candidate NeighborWindow, direction NeighborDirection) *measurement {
	if candidate.ID == origin.ID || !candidate.Frame.HasArea() {
		return nil
	}
	displayRank := displayRankOf(origin, candidate)
	if origin.IsFullScreen && displayRank == 0 {
		return nil
	}
	originCenter := origin.Frame.Center()
	candidateCenter := candidate.Frame.Center()
	if math.Hypot(originCenter.X-candidateCenter.X, originCenter.Y-candidateCenter.Y) <= coincidentCenterTolerance {
		return stackedMeasurement(origin, candidate, direction, displayRank)
	}
	return geometricMeasurement(origin, candidate, direction, displayRank)
}

// compareNumbers orders by value; ties return zero so the next key decides.
func compareNumbers(a, b float64) int {
	return compareFloats(a, b)
}

func compareIDs(first, second string) int {
	switch {
	case first == second:
		return 0
	case first < second:
		return -1
	}
	return 1
}

// compareNearest prefers the same display, then the lower cone score, then the front-most window,
// then the smaller ID.
func compareNearest(first, second *measurement) int {
	for _, step := range []int{
		first.displayRank - second.displayRank,
		compareNumbers(first.score, second.score),
		compareNumbers(first.stackStep, second.stackStep),
		first.zIndex - second.zIndex,
	} {
		if step != 0 {
			return step
		}
	}
	return compareIDs(first.id, second.id)
}

// compareFarthest prefers the same display, then the greatest axial distance (for wrapping), then
// the smallest sideways offset, then the front-most window, then the smaller ID.
func compareFarthest(first, second *measurement) int {
	for _, step := range []int{
		first.displayRank - second.displayRank,
		compareNumbers(second.axialDistance, first.axialDistance),
		compareNumbers(first.perpendicularOffset, second.perpendicularOffset),
		first.zIndex - second.zIndex,
	} {
		if step != 0 {
			return step
		}
	}
	return compareIDs(first.id, second.id)
}

// bestOf returns the minimum under compare, or nil when the list is empty.
func bestOf(measurements []*measurement, compare func(first, second *measurement) int) *measurement {
	var best *measurement
	for _, candidate := range measurements {
		if best == nil || compare(candidate, best) < 0 {
			best = candidate
		}
	}
	return best
}

// NearestWindowInDirection returns the ID of the window the origin should move focus to in a
// direction, and whether one exists. Windows that do not have area, are hidden behind the origin, or
// lie on the wrong side are never returned. With Wrap set and no window ahead, the far side is used.
func NearestWindowInDirection(origin NeighborWindow, candidates []NeighborWindow, direction NeighborDirection, options NeighborOptions) (string, bool) {
	if !origin.Frame.HasArea() {
		return "", false
	}
	var ahead, behind []*measurement
	for _, candidate := range candidates {
		entry := measureNeighbor(origin, candidate, direction)
		if entry == nil {
			continue
		}
		if entry.ahead {
			ahead = append(ahead, entry)
		} else {
			behind = append(behind, entry)
		}
	}
	if nearest := bestOf(ahead, compareNearest); nearest != nil || !options.Wrap {
		if nearest == nil {
			return "", false
		}
		return nearest.id, true
	}
	farthest := bestOf(behind, compareFarthest)
	if farthest == nil {
		return "", false
	}
	return farthest.id, true
}
