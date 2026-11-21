package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// applyLayout places the window by frame arithmetic. A repeated half command continues its cycle.
func (s *Service) applyLayout(ctx context.Context, req request, window writable) (Result, error) {
	previous := s.history.read(window.record.Key)
	layout, cycle, err := geometry.ResolveCycleStep(req.layout, lastAppliedOf(previous), window.window, req.options)
	if err != nil {
		return Result{}, newFailure(ReasonInvalidSpec, "")
	}
	requested, _, err := geometry.Resolve(layout, window.display, window.window, req.options, nil)
	if err != nil {
		return Result{}, newFailure(ReasonInvalidSpec, "")
	}
	result, failed := s.place(ctx, placementOf(req, window, window.display.WorkArea), requested)
	if failed != nil {
		return Result{}, failed
	}
	s.history.write(window.record.Key, geometry.RememberApplied(previous, window.window, result.Frame, result.Changed, cycle))
	return result, nil
}

func lastAppliedOf(previous *geometry.History) *geometry.AppliedLayout {
	if previous == nil {
		return nil
	}
	return previous.LastApplied
}

func placementOf(req request, window writable, workArea geometry.Rect) placement {
	return placement{
		pid:      req.target.PID,
		key:      window.record.Key,
		original: window.window.Frame,
		workArea: workArea,
	}
}
