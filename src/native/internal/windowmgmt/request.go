package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// request is one layout command on one window. windowKey names a listed window; empty means the
// focused window of the target app, which is the only case that checks focus against source.
type request struct {
	layout    geometry.Layout
	options   geometry.Options
	target    Target
	source    Source
	windowKey string
}

func (r request) targetsFocusedWindow() bool {
	return r.windowKey == ""
}

// runRequest runs the command on its target process queue and forgets the history of a process that
// has gone away.
func (s *Service) runRequest(ctx context.Context, req request) (Result, error) {
	return runQueued(ctx, s.queue, req.target.PID, func(ctx context.Context) (Result, error) {
		result, err := s.runOperation(ctx, req)
		if isTargetGone(err) {
			s.history.dropProcess(req.target.PID)
		}
		return result, err
	})
}

// runOperation checks focus, resolves the window, and dispatches the layout to its operation.
func (s *Service) runOperation(ctx context.Context, req request) (Result, error) {
	if req.targetsFocusedWindow() && req.source == SourcePalette {
		if failed := s.restorePaletteFocus(ctx, req.target); failed != nil {
			return Result{}, failed
		}
	}
	window, failed := s.resolveWritable(ctx, req.target, req.windowKey, req.layout.Kind == geometry.KindToggleFullScreen)
	if failed != nil {
		return Result{}, failed
	}
	if req.targetsFocusedWindow() && req.source == SourceShortcut {
		if failed := s.guardShortcutFocus(ctx, req.target); failed != nil {
			return Result{}, failed
		}
	}
	return s.dispatch(ctx, req, window)
}

func (s *Service) dispatch(ctx context.Context, req request, window writable) (Result, error) {
	switch req.layout.Kind {
	case geometry.KindRestore:
		return s.applyRestore(ctx, req, window)
	case geometry.KindDisplay:
		return s.applyDisplayMove(ctx, req, window)
	case geometry.KindToggleFullScreen:
		return s.applyFullScreenToggle(ctx, req, window)
	case geometry.KindFraction, geometry.KindCenter, geometry.KindMaximizeWidth, geometry.KindMaximizeHeight,
		geometry.KindScaledCenter, geometry.KindEdge, geometry.KindResizeStep, geometry.KindExact:
		return s.applyLayout(ctx, req, window)
	}
	return Result{}, newFailure(ReasonInvalidSpec, "")
}
