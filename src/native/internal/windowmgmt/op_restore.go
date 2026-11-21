package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// applyRestore puts the window back where it was before Nevermind first moved it. History is forgotten
// only after the window reached the restore frame.
func (s *Service) applyRestore(ctx context.Context, req request, window writable) (Result, error) {
	previous := s.history.read(window.record.Key)
	if previous == nil || previous.RestoreFrame == nil {
		return Result{}, newFailure(ReasonNoHistory, "")
	}
	display, err := geometry.RestoreDisplay(window.displays, previous)
	if err != nil {
		return Result{}, newFailure(ReasonNoHistory, "")
	}
	requested, _, err := geometry.Resolve(geometry.Layout{Kind: geometry.KindRestore}, display, window.window, req.options, previous)
	if err != nil {
		return Result{}, newFailure(ReasonNoHistory, "")
	}
	result, failed := s.place(ctx, placementOf(req, window, display.WorkArea), requested)
	if failed != nil {
		return Result{}, failed
	}
	s.history.forget(window.record.Key)
	return result, nil
}
