package windowmgmt

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// extensionRequest turns the api's layout and options into the engine's. A layout the engine cannot read, such as a
// fraction without its rectangle, fails as invalid-spec before any window is touched.
func extensionRequest(layout api.WindowLayout, options api.WindowManagementOptions) (geometry.Layout, geometry.Options, *Failure) {
	geometryLayout, err := layoutOf(layout)
	if err != nil {
		return geometry.Layout{}, geometry.Options{}, newFailure(ReasonInvalidSpec, err.Error())
	}
	return geometryLayout, optionsOf(options), nil
}

func layoutOf(layout api.WindowLayout) (geometry.Layout, error) {
	out := geometry.Layout{
		Kind:           geometry.LayoutKind(layout.Kind),
		CycleGroup:     geometry.Edge(layout.CycleGroup),
		WidthFraction:  layout.WidthFraction,
		HeightFraction: layout.HeightFraction,
		Edge:           geometry.Edge(layout.Edge),
		Direction:      geometry.Direction(layout.Direction),
		Step:           layout.Step,
	}
	if layout.MaxWidth > 0 {
		out.MaxWidth = &layout.MaxWidth
	}
	if layout.MaxHeight > 0 {
		out.MaxHeight = &layout.MaxHeight
	}
	if layout.Kind == api.LayoutFraction {
		if layout.Fraction == nil {
			return geometry.Layout{}, errors.New("fraction layout needs a fraction rect")
		}
		out.X, out.Y, out.Width, out.Height = layout.Fraction.X, layout.Fraction.Y, layout.Fraction.Width, layout.Fraction.Height
	}
	if layout.Kind == api.LayoutExact {
		if layout.Exact == nil {
			return geometry.Layout{}, errors.New("exact layout needs an exact frame")
		}
		out.ExactX = dimensionOf(layout.Exact.X)
		out.ExactY = dimensionOf(layout.Exact.Y)
		out.ExactWidth = dimensionOf(layout.Exact.Width)
		out.ExactHeight = dimensionOf(layout.Exact.Height)
	}
	return out, nil
}

func dimensionOf(dimension *api.Dimension) *geometry.Dimension {
	if dimension == nil {
		return nil
	}
	return &geometry.Dimension{Value: dimension.Value, Unit: geometry.DimensionUnit(dimension.Unit)}
}

// optionsOf starts from the palette defaults and applies the options an extension sets. The engine checks the values.
func optionsOf(options api.WindowManagementOptions) geometry.Options {
	out := geometry.DefaultOptions()
	if options.OuterGap != nil {
		out.OuterGap = geometry.EdgeGaps{Top: float64(options.OuterGap.Top), Right: float64(options.OuterGap.Right), Bottom: float64(options.OuterGap.Bottom), Left: float64(options.OuterGap.Left)}
	}
	if options.InnerGap != nil {
		out.InnerGap = float64(*options.InnerGap)
	}
	if options.ApplyGapToMaximize != nil {
		out.ApplyGapToMaximize = *options.ApplyGapToMaximize
	}
	if options.Cycle != nil {
		out.Cycle = *options.Cycle
	}
	if options.CycleOrder != "" {
		out.CycleOrder = geometry.CycleOrder(options.CycleOrder)
	}
	if options.DisplayOrder != "" {
		out.DisplayOrder = geometry.DisplayOrder(options.DisplayOrder)
	}
	if options.DisplayMoveSizing != "" {
		out.DisplayMoveSizing = geometry.DisplayMoveSizing(options.DisplayMoveSizing)
	}
	return out
}

func neighborDirectionOf(direction string) (geometry.NeighborDirection, bool) {
	switch geometry.NeighborDirection(direction) {
	case geometry.NeighborLeft, geometry.NeighborRight, geometry.NeighborUp, geometry.NeighborDown:
		return geometry.NeighborDirection(direction), true
	}
	return "", false
}

// applyResultOf reports a layout the way the API does: OK for an exact or partial placement, OK false with a reason
// for everything else. Only a failure that is not a *Failure is returned as a Go error.
func applyResultOf(result Result, err error) (api.ApplyResult, error) {
	if err != nil {
		failure, ok := asFailure(err)
		if !ok {
			return api.ApplyResult{}, err
		}
		return failedApply(failure), nil
	}
	out := api.ApplyResult{OK: true, Outcome: outcomeName(result.Outcome), Changed: result.Changed, Frame: apiRect(result.Frame)}
	if result.Outcome == geometry.Partial {
		out.Reason = result.Message
	}
	return out, nil
}

func failedApply(failure *Failure) api.ApplyResult {
	return api.ApplyResult{OK: false, Reason: string(failure.Reason), Message: failure.Error()}
}

func focusResultOf(err error) (api.FocusResult, error) {
	if err == nil {
		return api.FocusResult{OK: true}, nil
	}
	failure, ok := asFailure(err)
	if !ok {
		return api.FocusResult{}, err
	}
	return api.FocusResult{OK: false, Reason: string(failure.Reason), Message: failure.Error()}, nil
}

func asFailure(err error) (*Failure, bool) {
	var failure *Failure
	return failure, errors.As(err, &failure)
}

func outcomeName(outcome geometry.Outcome) string {
	switch outcome {
	case geometry.Applied:
		return "exact"
	case geometry.Partial:
		return "partial"
	}
	return "failed"
}
