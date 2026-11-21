package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// applyFullScreenToggle enters or leaves full screen. The new state is read back before success is
// reported, and the repeated-half cycle ends because the window no longer sits in a layout.
func (s *Service) applyFullScreenToggle(ctx context.Context, req request, window writable) (Result, error) {
	wantFullScreen := !window.window.IsFullScreen
	if _, err := s.windows.SetFullScreen(ctx, req.target.PID, window.record.Key, wantFullScreen); err != nil {
		return Result{}, failureOf(err)
	}
	after, failed := s.readTargetWindow(ctx, req.target, window.record.Key)
	if failed != nil {
		return Result{}, failed
	}
	if after.FullScreen != wantFullScreen {
		return Result{}, newFailure(ReasonUnapplied, "")
	}
	s.endCycle(window.record.Key)
	return Result{Outcome: geometry.Applied, Changed: true, Frame: rectFromPlatform(after.Frame)}, nil
}

func (s *Service) endCycle(windowKey string) {
	previous := s.history.read(windowKey)
	if previous == nil || previous.LastApplied == nil {
		return
	}
	s.history.write(windowKey, geometry.ForgetCycleGroup(*previous))
}
