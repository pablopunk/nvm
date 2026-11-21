package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// applyDisplayMove moves the window to the next or previous display, keeping its place in the work area
// according to the display move sizing option.
func (s *Service) applyDisplayMove(ctx context.Context, req request, window writable) (Result, error) {
	previous := s.history.read(window.record.Key)
	requested, next, err := geometry.MoveWindowToAdjacentDisplay(window.displays, window.window, req.layout.Direction, req.options)
	if err != nil {
		return Result{}, newFailure(ReasonSingleDisplay, "")
	}
	result, failed := s.place(ctx, placementOf(req, window, next.WorkArea), requested)
	if failed != nil {
		return Result{}, failed
	}
	s.history.write(window.record.Key, geometry.RememberApplied(previous, window.window, result.Frame, result.Changed, geometry.CycleRecord{}))
	return result, nil
}
