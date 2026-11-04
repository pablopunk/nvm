package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// ApplyToWindow runs a layout on a window named by a handle from ListWindows. The command does not need
// focus: the window is acted on directly. A handle whose window is gone is forgotten.
func (s *Service) ApplyToWindow(ctx context.Context, handle string, layout geometry.Layout, options geometry.Options) (Result, error) {
	target, ok := s.handles.resolve(handle)
	if !ok {
		return Result{}, newFailure(ReasonUnknownWindow, "")
	}
	if failed := validateRequest(layout, options); failed != nil {
		return Result{}, failed
	}
	if !s.Available() {
		return Result{}, newFailure(ReasonUnsupported, "")
	}
	if target.PID == s.selfPID {
		return Result{}, newFailure(ReasonNoTarget, messageSelfTarget)
	}
	result, err := s.runRequest(ctx, request{
		layout:    layout,
		options:   options,
		target:    Target{PID: target.PID, BundleID: target.BundleID},
		windowKey: target.Key,
	})
	s.forgetIfLost(handle, err)
	return result, err
}

// FocusWindow brings a window named by a handle to the front.
func (s *Service) FocusWindow(ctx context.Context, handle string) error {
	target, ok := s.handles.resolve(handle)
	if !ok {
		return newFailure(ReasonUnknownWindow, "")
	}
	if !s.Available() {
		return newFailure(ReasonUnsupported, "")
	}
	_, err := runQueued(ctx, s.queue, target.PID, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, s.raise(ctx, target.PID, target.Key)
	})
	s.forgetIfLost(handle, err)
	return err
}

// raise brings a window to the front after the access check. Errors are returned as *Failure.
func (s *Service) raise(ctx context.Context, pid int, key string) error {
	if failed := s.accessFailure(ctx); failed != nil {
		return failed
	}
	if err := s.windows.Raise(ctx, pid, key); err != nil {
		return failureOf(err)
	}
	return nil
}

func (s *Service) forgetIfLost(handle string, err error) {
	if isTargetGone(err) {
		s.handles.forget(handle)
	}
}
