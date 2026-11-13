package geometry

import (
	"encoding/json"
	"os"
	"testing"
)

// golden_test replays testdata/golden.json, which the TypeScript geometry (1.x/electron/os/window-*.ts)
// produced over seeded random inputs. Each case states the TypeScript answer; the Go port must agree.

type goldenRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (g goldenRect) rect() Rect {
	return Rect(g)
}

type goldenDisplay struct {
	ID        string     `json:"id"`
	IsPrimary bool       `json:"isPrimary"`
	Frame     goldenRect `json:"frame"`
	WorkArea  goldenRect `json:"workArea"`
}

func (g goldenDisplay) display() Display {
	return Display{ID: g.ID, Main: g.IsPrimary, Scale: 1, Frame: g.Frame.rect(), WorkArea: g.WorkArea.rect()}
}

type goldenEdges struct {
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

type goldenOptions struct {
	OuterGap           goldenEdges `json:"outerGap"`
	InnerGap           float64     `json:"innerGap"`
	ApplyGapToMaximize bool        `json:"applyGapToMaximize"`
	CycleOrder         string      `json:"cycleOrder"`
	DisplayOrder       string      `json:"displayOrder"`
	DisplayMoveSizing  string      `json:"displayMoveSizing"`
}

// options converts golden options; fields absent from the golden keep their defaults.
func (g goldenOptions) options() Options {
	options := DefaultOptions()
	options.OuterGap = EdgeGaps{Top: g.OuterGap.Top, Right: g.OuterGap.Right, Bottom: g.OuterGap.Bottom, Left: g.OuterGap.Left}
	options.InnerGap = g.InnerGap
	options.ApplyGapToMaximize = g.ApplyGapToMaximize
	if g.CycleOrder != "" {
		options.CycleOrder = CycleOrder(g.CycleOrder)
	}
	if g.DisplayOrder != "" {
		options.DisplayOrder = DisplayOrder(g.DisplayOrder)
	}
	if g.DisplayMoveSizing != "" {
		options.DisplayMoveSizing = DisplayMoveSizing(g.DisplayMoveSizing)
	}
	return options
}

type goldenDimension struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type goldenExact struct {
	X      *goldenDimension `json:"x"`
	Y      *goldenDimension `json:"y"`
	Width  *goldenDimension `json:"width"`
	Height *goldenDimension `json:"height"`
}

// goldenLayout is the flattened TypeScript WindowLayoutSpec.
type goldenLayout struct {
	Kind           string       `json:"kind"`
	X              float64      `json:"x"`
	Y              float64      `json:"y"`
	Width          float64      `json:"width"`
	Height         float64      `json:"height"`
	CycleGroup     string       `json:"cycleGroup"`
	WidthFraction  float64      `json:"widthFraction"`
	HeightFraction float64      `json:"heightFraction"`
	MaxWidth       *float64     `json:"maxWidth"`
	MaxHeight      *float64     `json:"maxHeight"`
	Edge           string       `json:"edge"`
	Direction      string       `json:"direction"`
	Step           float64      `json:"step"`
	Exact          *goldenExact `json:"exact"`
}

func goldenDim(d *goldenDimension) *Dimension {
	if d == nil {
		return nil
	}
	return &Dimension{Value: d.Value, Unit: DimensionUnit(d.Unit)}
}

func (g goldenLayout) layout() Layout {
	layout := Layout{
		Kind: LayoutKind(g.Kind),
		X:    g.X, Y: g.Y, Width: g.Width, Height: g.Height,
		CycleGroup:    Edge(g.CycleGroup),
		WidthFraction: g.WidthFraction, HeightFraction: g.HeightFraction,
		MaxWidth: g.MaxWidth, MaxHeight: g.MaxHeight,
		Edge:      Edge(g.Edge),
		Direction: Direction(g.Direction),
		Step:      g.Step,
	}
	if g.Exact != nil {
		layout.ExactX = goldenDim(g.Exact.X)
		layout.ExactY = goldenDim(g.Exact.Y)
		layout.ExactWidth = goldenDim(g.Exact.Width)
		layout.ExactHeight = goldenDim(g.Exact.Height)
	}
	return layout
}

type goldenWindow struct {
	Frame       goldenRect `json:"frame"`
	IsResizable bool       `json:"isResizable"`
}

type goldenLayoutCase struct {
	Preset   string       `json:"preset"`
	Spec     goldenLayout `json:"spec"`
	Display  int          `json:"display"`
	Options  int          `json:"options"`
	Window   goldenWindow `json:"window"`
	Expected goldenRect   `json:"expected"`
}

type goldenPickCase struct {
	Displays []goldenDisplay `json:"displays"`
	Frame    goldenRect      `json:"frame"`
	Expected string          `json:"expected"`
}

type goldenAdjacentCase struct {
	Displays  []goldenDisplay `json:"displays"`
	From      string          `json:"from"`
	Direction string          `json:"direction"`
	Order     string          `json:"order"`
	Expected  *string         `json:"expected"`
}

type goldenMoveEnd struct {
	ID       string     `json:"id"`
	Frame    goldenRect `json:"frame"`
	WorkArea goldenRect `json:"workArea"`
}

type goldenMoveCase struct {
	From     goldenMoveEnd `json:"from"`
	To       goldenMoveEnd `json:"to"`
	Frame    goldenRect    `json:"frame"`
	Sizing   string        `json:"sizing"`
	Expected goldenRect    `json:"expected"`
}

type goldenCycleCase struct {
	Group    string       `json:"group"`
	Order    string       `json:"order"`
	Expected []goldenRect `json:"expected"`
}

type goldenCycleStep struct {
	Spec         goldenLayout  `json:"spec"`
	Last         *goldenLast   `json:"last"`
	Current      goldenRect    `json:"current"`
	CycleEnabled bool          `json:"cycleEnabled"`
	CycleOrder   string        `json:"cycleOrder"`
	Expected     goldenStepOut `json:"expected"`
}

type goldenLast struct {
	Frame      goldenRect `json:"frame"`
	CycleGroup string     `json:"cycleGroup"`
	CycleIndex int        `json:"cycleIndex"`
}

type goldenStepOut struct {
	Spec       goldenRect `json:"spec"`
	CycleGroup *string    `json:"cycleGroup"`
	CycleIndex int        `json:"cycleIndex"`
}

type goldenNeighborWindow struct {
	ID           string     `json:"id"`
	Frame        goldenRect `json:"frame"`
	ZIndex       int        `json:"zIndex"`
	DisplayID    string     `json:"displayId"`
	IsFullScreen bool       `json:"isFullScreen"`
}

func (g goldenNeighborWindow) window() NeighborWindow {
	return NeighborWindow{ID: g.ID, Frame: g.Frame.rect(), ZIndex: g.ZIndex, DisplayID: g.DisplayID, IsFullScreen: g.IsFullScreen}
}

type goldenNeighborCase struct {
	Origin     goldenNeighborWindow   `json:"origin"`
	Candidates []goldenNeighborWindow `json:"candidates"`
	Direction  string                 `json:"direction"`
	Wrap       bool                   `json:"wrap"`
	Expected   *string                `json:"expected"`
}

type goldenValidationCase struct {
	Spec     goldenLayout  `json:"spec"`
	Options  goldenOptions `json:"options"`
	Expected *string       `json:"expected"`
}

type goldenFile struct {
	Displays        []goldenDisplay        `json:"displays"`
	Options         []goldenOptions        `json:"options"`
	PresetCases     []goldenLayoutCase     `json:"presetCases"`
	ExactCases      []goldenLayoutCase     `json:"exactCases"`
	PickCases       []goldenPickCase       `json:"pickCases"`
	AdjacentCases   []goldenAdjacentCase   `json:"adjacentCases"`
	MoveCases       []goldenMoveCase       `json:"moveCases"`
	CycleCases      []goldenCycleCase      `json:"cycleCases"`
	CycleStepCases  []goldenCycleStep      `json:"cycleStepCases"`
	NeighborCases   []goldenNeighborCase   `json:"neighborCases"`
	ValidationCases []goldenValidationCase `json:"validationCases"`
}

const goldenPath = "testdata/golden.json"

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read %s: %v", goldenPath, err)
	}
	var golden goldenFile
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("parse %s: %v", goldenPath, err)
	}
	return golden
}
