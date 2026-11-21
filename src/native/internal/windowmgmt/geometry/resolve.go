package geometry

// Meta describes how a resolved frame was produced, for the engine and the palette.
type Meta struct {
	// Anchored is true when the window cannot resize, so it kept its size and was aligned in the zone.
	Anchored bool
}

// Resolve computes the frame a layout asks for on display, given the window's current frame.
// Frame math runs against display.WorkArea, so the menu bar and Dock are respected. The result is
// already fitted for windows that cannot resize (FitRequestedFrame).
//
// Restore resolves from previous.RestoreFrame and expects display to be the display that contains
// that stored frame (RestoreDisplay). Display moves, full-screen toggles and invalid kinds are not
// frame arithmetic and return an error; use MoveWindowToAdjacentDisplay for display moves.
func Resolve(layout Layout, display Display, window WindowFrame, options Options, previous *History) (Rect, Meta, error) {
	if layout.Kind == KindRestore {
		return resolveRestore(display, window, previous)
	}
	if IsServiceHandled(layout.Kind) {
		return Rect{}, Meta{}, ServiceHandledError(layout.Kind)
	}
	if !display.WorkArea.HasArea() {
		return Rect{}, Meta{}, ErrNoWorkArea
	}
	computed, err := computeTargetFrame(layout, display.WorkArea, window.Frame, options)
	if err != nil {
		return Rect{}, Meta{}, err
	}
	return FitRequestedFrame(computed, window, display.WorkArea), Meta{Anchored: !window.IsResizable}, nil
}

func resolveRestore(display Display, window WindowFrame, previous *History) (Rect, Meta, error) {
	if previous == nil || previous.RestoreFrame == nil {
		return Rect{}, Meta{}, ErrNoHistory
	}
	if !display.WorkArea.HasArea() {
		return Rect{}, Meta{}, ErrNoWorkArea
	}
	fitted := FitFrameInside(*previous.RestoreFrame, display.WorkArea)
	return FitRequestedFrame(fitted, window, display.WorkArea), Meta{Anchored: !window.IsResizable}, nil
}

// computeTargetFrame is computeTargetFrame from window-layouts.ts: the un-fitted target frame.
func computeTargetFrame(layout Layout, workArea, current Rect, options Options) (Rect, error) {
	if IsServiceHandled(layout.Kind) {
		return Rect{}, ServiceHandledError(layout.Kind)
	}
	area := axesOf(workArea)
	currentAxes := axesOf(current)
	switch layout.Kind {
	case KindFraction:
		return fractionFrameInWorkArea(layout, area, options), nil
	case KindCenter:
		return frameFromAxes(axes{
			horizontal: centeredSpan(area.horizontal, currentAxes.horizontal.length),
			vertical:   centeredSpan(area.vertical, currentAxes.vertical.length),
		}), nil
	case KindMaximizeWidth:
		return maximizeWidthFrame(area, currentAxes, options), nil
	case KindMaximizeHeight:
		return maximizeHeightFrame(area, currentAxes, options), nil
	case KindScaledCenter:
		return scaledCenterFrame(layout, area), nil
	case KindEdge:
		return edgeFrame(layout.Edge, area, currentAxes, options.OuterGap), nil
	case KindResizeStep:
		return resizedFrame(layout, area, currentAxes), nil
	case KindExact:
		return exactFrame(layout, workArea, current), nil
	}
	return Rect{}, ErrUnhandledKind
}

func fractionFrameInWorkArea(layout Layout, area axes, options Options) Rect {
	insets := insetsForFraction(unitRect{x: layout.X, y: layout.Y, width: layout.Width, height: layout.Height}, options)
	return frameFromAxes(axes{
		horizontal: fractionSpanInArea(area.horizontal, layout.X, layout.Width, insets.horizontal),
		vertical:   fractionSpanInArea(area.vertical, layout.Y, layout.Height, insets.vertical),
	})
}

func scaledCenterFrame(layout Layout, area axes) Rect {
	return frameFromAxes(axes{
		horizontal: centeredSpan(area.horizontal, scaledLength(area.horizontal.length, layout.WidthFraction, layout.MaxWidth)),
		vertical:   centeredSpan(area.vertical, scaledLength(area.vertical.length, layout.HeightFraction, layout.MaxHeight)),
	})
}

func edgeFrame(edge Edge, area, current axes, outer EdgeGaps) Rect {
	alignment := edgeAlignments[edge]
	gap := outer.Edge(edge)
	return frameFromAxes(axes{
		horizontal: alignedSpan(area.horizontal, current.horizontal, alignment.horizontal, gap),
		vertical:   alignedSpan(area.vertical, current.vertical, alignment.vertical, gap),
	})
}

func maximizeWidthFrame(area, current axes, options Options) Rect {
	return frameFromAxes(axes{
		horizontal: insetSpan(area.horizontal, maximizedAxisInsets(options, options.OuterGap.Left, options.OuterGap.Right)),
		vertical:   placeSpan(area.vertical, current.vertical.length, current.vertical.origin),
	})
}

func maximizeHeightFrame(area, current axes, options Options) Rect {
	return frameFromAxes(axes{
		horizontal: placeSpan(area.horizontal, current.horizontal.length, current.horizontal.origin),
		vertical:   insetSpan(area.vertical, maximizedAxisInsets(options, options.OuterGap.Top, options.OuterGap.Bottom)),
	})
}

// resizedFrame grows or shrinks around the window center by a fraction of the work area.
func resizedFrame(layout Layout, area, current axes) Rect {
	direction := -1.0
	if layout.Direction == DirectionLarger {
		direction = 1
	}
	return frameFromAxes(axes{
		horizontal: resizedSpan(area.horizontal, current.horizontal, direction*layout.Step*area.horizontal.length),
		vertical:   resizedSpan(area.vertical, current.vertical, direction*layout.Step*area.vertical.length),
	})
}
