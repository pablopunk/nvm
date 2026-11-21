package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// placement is the window a frame is written to, with the frame it had before and the display it must
// stay inside.
type placement struct {
	pid      int
	key      string
	original geometry.Rect
	workArea geometry.Rect
}

// place writes a frame and reads back what the window reached. Success is claimed only for a frame the
// window actually reached (geometry.Verify). A partial write that left the window outside its display
// gets one position-only correction.
func (s *Service) place(ctx context.Context, at placement, requested geometry.Rect) (Result, *Failure) {
	observed, err := s.setFrame(ctx, at, requested, false)
	if err != nil {
		return Result{}, failureOf(err)
	}
	verdict := geometry.Verify(requested, observed, at.original, geometry.Size{})
	if verdict.Outcome == geometry.Failed {
		return Result{}, newFailure(ReasonUnapplied, verdict.Message)
	}
	if !geometry.NeedsOverflowCorrection(verdict.Outcome, observed, at.workArea) {
		return resultOf(verdict, observed, at.original), nil
	}
	return s.correctOverflow(ctx, at, requested, observed)
}

func (s *Service) correctOverflow(ctx context.Context, at placement, requested, observed geometry.Rect) (Result, *Failure) {
	correction := geometry.OverflowCorrectionFrame(requested, observed, at.workArea)
	reached, err := s.setFrame(ctx, at, correction, true)
	if err != nil {
		reached = observed
	}
	final := geometry.Verify(requested, reached, at.original, geometry.Size{})
	if final.Outcome == geometry.Failed {
		return Result{}, newFailure(ReasonUnapplied, final.Message)
	}
	result := resultOf(final, reached, at.original)
	if err != nil && final.Outcome == geometry.Partial {
		result.Message = geometry.MessageCorrectionFailed
	}
	return result, nil
}

func (s *Service) setFrame(ctx context.Context, at placement, frame geometry.Rect, positionOnly bool) (geometry.Rect, error) {
	reached, err := s.windows.SetFrame(ctx, platform.WindowWrite{
		PID:          at.pid,
		Key:          at.key,
		Frame:        platformFrameOf(frame),
		PositionOnly: positionOnly,
	})
	if err != nil {
		return geometry.Rect{}, err
	}
	return rectFromPlatform(reached), nil
}

func resultOf(verdict geometry.Verdict, frame, original geometry.Rect) Result {
	result := Result{
		Outcome: verdict.Outcome,
		Changed: !geometry.FramesMatch(frame, original, 0),
		Frame:   frame,
	}
	if verdict.Outcome == geometry.Partial {
		result.Message = verdict.Message
	}
	return result
}
