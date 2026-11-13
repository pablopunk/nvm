package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// Extension is the window engine as the extension services see it: the same operations in the api's wire types.
// Runtime failures come back as results with OK false, the way the TypeScript API reports them. Invocations names
// the app a call acts on: the palette's target while the palette is open, the app in front for a shortcut.
type Extension struct {
	engine      *Service
	invocations func(context.Context) Invocation
}

// NewExtension returns the extension view of engine.
func NewExtension(engine *Service, invocations func(context.Context) Invocation) Extension {
	return Extension{engine: engine, invocations: invocations}
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
	return focusResultOf(x.engine.FocusWindow(ctx, string(handle)))
}

// FocusNeighbor raises the nearest window in a direction from the invocation's target window.
func (x Extension) FocusNeighbor(ctx context.Context, direction string, wrap bool) (api.FocusResult, error) {
	neighborDirection, ok := neighborDirectionOf(direction)
	if !ok {
		return api.FocusResult{OK: false, Reason: string(ReasonInvalidSpec), Message: geometry.MessageInvalidSpec}, nil
	}
	return focusResultOf(x.engine.FocusNeighbor(ctx, neighborDirection, geometry.NeighborOptions{Wrap: wrap}, x.invocations(ctx)))
}

// Apply lays out the focused window of the invocation's target app.
func (x Extension) Apply(ctx context.Context, layout api.WindowLayout, options api.WindowManagementOptions) (api.ApplyResult, error) {
	geometryLayout, geometryOptions, failure := extensionRequest(layout, options)
	if failure != nil {
		return failedApply(failure), nil
	}
	result, err := x.engine.Apply(ctx, geometryLayout, geometryOptions, x.invocations(ctx))
	return applyResultOf(result, err)
}

// ApplyTo places the listed window. The layout and options are the ones Apply takes.
func (x Extension) ApplyTo(ctx context.Context, handle api.WindowHandle, layout api.WindowLayout, options api.WindowManagementOptions) (api.ApplyResult, error) {
	geometryLayout, geometryOptions, failure := extensionRequest(layout, options)
	if failure != nil {
		return failedApply(failure), nil
	}
	result, err := x.engine.ApplyToWindow(ctx, string(handle), geometryLayout, geometryOptions)
	return applyResultOf(result, err)
}

func apiRect(rect geometry.Rect) api.Rect {
	return api.Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}
}
