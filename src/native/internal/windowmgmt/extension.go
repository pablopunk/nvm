package windowmgmt

import (
	"context"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// Extension is the window engine as the extension services see it: the same operations in the api's wire types.
// Runtime failures come back as results with OK false, the way the TypeScript API reports them.
type Extension struct {
	engine *Service
}

// NewExtension returns the extension view of engine.
func NewExtension(engine *Service) Extension {
	return Extension{engine: engine}
}

// Displays lists the connected displays with their frames and work areas.
func (x Extension) Displays(ctx context.Context) ([]api.DisplayInfo, error) {
	displays, err := x.engine.ListDisplays(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.DisplayInfo, 0, len(displays))
	for _, display := range displays {
		out = append(out, api.DisplayInfo{ID: display.ID, Primary: display.Main, Frame: apiRect(display.Frame), WorkArea: apiRect(display.WorkArea)})
	}
	return out, nil
}

// Windows lists the windows that match the query. Access that the system denies lists nothing, as the API documents.
func (x Extension) Windows(ctx context.Context, query api.WindowQuery) ([]api.WindowInfo, error) {
	listed, err := x.engine.ListWindows(ctx, WindowQuery{AppID: query.AppID, AppName: query.AppName, Title: query.Title})
	if failure, ok := asFailure(err); ok && failure.Reason == ReasonDenied {
		return []api.WindowInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]api.WindowInfo, 0, len(listed))
	for _, window := range listed {
		out = append(out, api.WindowInfo{
			Handle: api.WindowHandle(window.Handle), AppID: window.AppID, AppName: window.AppName, Title: window.Title,
			Frame: apiRect(window.Frame), DisplayID: window.DisplayID, FullScreen: window.FullScreen, Minimized: window.Minimized,
		})
	}
	return out, nil
}

// Focus raises the listed window. A stale or refused handle is a result with OK false.
func (x Extension) Focus(ctx context.Context, handle api.WindowHandle) (api.FocusResult, error) {
	err := x.engine.FocusWindow(ctx, string(handle))
	if err == nil {
		return api.FocusResult{OK: true}, nil
	}
	failure, ok := asFailure(err)
	if !ok {
		return api.FocusResult{}, err
	}
	return api.FocusResult{OK: false, Reason: string(failure.Reason), Message: failure.Error()}, nil
}

// ApplyTo places the listed window. Only fraction layouts are served to extensions for now; the window commands of
// the palette cover the rest.
func (x Extension) ApplyTo(ctx context.Context, handle api.WindowHandle, layout api.WindowLayout, options api.WindowManagementOptions) (api.ApplyResult, error) {
	spec, ok := fractionLayoutOf(layout)
	if !ok {
		return api.ApplyResult{OK: false, Reason: string(ReasonUnsupported), Message: "this layout is not available to extensions yet"}, nil
	}
	result, err := x.engine.ApplyToWindow(ctx, string(handle), spec, geometryOptionsOf(options))
	if err != nil {
		failure, ok := asFailure(err)
		if !ok {
			return api.ApplyResult{}, err
		}
		return api.ApplyResult{OK: false, Reason: string(failure.Reason), Message: failure.Error()}, nil
	}
	return api.ApplyResult{
		OK: result.Outcome == geometry.Applied, Outcome: outcomeName(result.Outcome), Changed: result.Changed,
		Frame: apiRect(result.Frame), Message: result.Message,
	}, nil
}

func asFailure(err error) (*Failure, bool) {
	var failure *Failure
	return failure, errors.As(err, &failure)
}

func fractionLayoutOf(layout api.WindowLayout) (geometry.Layout, bool) {
	if layout.Kind != api.LayoutFraction || layout.Fraction == nil {
		return geometry.Layout{}, false
	}
	fraction := layout.Fraction
	return geometry.Layout{Kind: geometry.KindFraction, X: fraction.X, Y: fraction.Y, Width: fraction.Width, Height: fraction.Height}, true
}

// geometryOptionsOf starts from the palette defaults and applies the options an extension may set.
func geometryOptionsOf(options api.WindowManagementOptions) geometry.Options {
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
	return out
}

func outcomeName(outcome geometry.Outcome) string {
	switch outcome {
	case geometry.Applied:
		return "applied"
	case geometry.Partial:
		return "partial"
	}
	return "failed"
}

func apiRect(rect geometry.Rect) api.Rect {
	return api.Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}
}
