package geometry

// cycleLength is the number of steps in a half cycle: half, two-thirds, third.
const cycleLength = 3

// CycleRecord is the cycle state a layout application leaves behind. Index is the step within the
// cycle; Group is empty when the application did not belong to a cycle.
type CycleRecord struct {
	Group Edge
	Index int
}

// cycleStepPresets names the three presets of each cycle group.
var cycleStepPresets = map[Edge]struct {
	half, twoThirds, third PresetID
}{
	EdgeLeft:   {half: "left-half", twoThirds: "first-two-thirds", third: "first-third"},
	EdgeRight:  {half: "right-half", twoThirds: "last-two-thirds", third: "last-third"},
	EdgeTop:    {half: "top-half", twoThirds: "top-two-thirds", third: "top-third"},
	EdgeBottom: {half: "bottom-half", twoThirds: "bottom-two-thirds", third: "bottom-third"},
}

// isContinuingCycle reports whether the window is still where the last cycle step put it, in the
// same group. A window moved by hand, or a command from another group, restarts the cycle.
func isContinuingCycle(last *AppliedLayout, current Rect, group Edge) bool {
	return last != nil && last.CycleGroup == group &&
		FramesMatch(last.Frame, current, CycleTolerancePoints)
}

// NextCycleIndex returns the step a repeated command should take: 0 when the cycle restarts,
// otherwise the step after the last one, wrapping after three.
func NextCycleIndex(last *AppliedLayout, current Rect, group Edge) int {
	if !isContinuingCycle(last, current, group) {
		return 0
	}
	return (last.CycleIndex + 1) % cycleLength
}

// CycleSequence returns the three fraction layouts of a cycle group in the order the options ask
// for. The layouts carry no cycle group of their own.
func CycleSequence(group Edge, order CycleOrder) ([]Layout, error) {
	steps, ok := cycleStepPresets[group]
	if !ok {
		return nil, ErrUnknownCycle
	}
	ids := []PresetID{steps.half, steps.twoThirds, steps.third}
	if order == CycleHalfThirdTwoThirds {
		ids = []PresetID{steps.half, steps.third, steps.twoThirds}
	}
	layouts := make([]Layout, 0, len(ids))
	for _, id := range ids {
		layout, _ := Preset(id)
		layout.CycleGroup = ""
		layouts = append(layouts, layout)
	}
	return layouts, nil
}

// ResolveCycleStep chooses the layout for a request. Only enabled, fraction layouts with a cycle
// group cycle; every other request runs as given at step 0.
func ResolveCycleStep(layout Layout, last *AppliedLayout, window WindowFrame, options Options) (Layout, CycleRecord, error) {
	if !options.Cycle || layout.Kind != KindFraction || layout.CycleGroup == "" {
		return layout, CycleRecord{}, nil
	}
	index := NextCycleIndex(last, window.Frame, layout.CycleGroup)
	sequence, err := CycleSequence(layout.CycleGroup, options.CycleOrder)
	if err != nil {
		return Layout{}, CycleRecord{}, err
	}
	return sequence[index], CycleRecord{Group: layout.CycleGroup, Index: index}, nil
}
