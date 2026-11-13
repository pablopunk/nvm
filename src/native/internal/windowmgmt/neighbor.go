package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// FocusNeighbor raises the nearest window in a direction from the target app's focused window.
func (s *Service) FocusNeighbor(ctx context.Context, direction geometry.NeighborDirection, options geometry.NeighborOptions, invocation Invocation) error {
	err := s.focusNeighborOf(ctx, direction, options, invocation)
	s.reportFailure("focus-neighbor", err)
	return err
}

func (s *Service) focusNeighborOf(ctx context.Context, direction geometry.NeighborDirection, options geometry.NeighborOptions, invocation Invocation) error {
	if !s.Available() {
		return newFailure(ReasonUnsupported, "")
	}
	if invocation.Target == nil {
		return newFailure(ReasonNoTarget, noTargetMessage(invocation.Source))
	}
	target := *invocation.Target
	if target.PID == s.selfPID {
		return newFailure(ReasonNoTarget, messageSelfTarget)
	}
	_, err := runQueued(ctx, s.queue, target.PID, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, s.focusNeighbor(ctx, direction, options, invocation.Source, target)
	})
	return err
}

func (s *Service) focusNeighbor(ctx context.Context, direction geometry.NeighborDirection, options geometry.NeighborOptions, source Source, target Target) error {
	if failed := s.accessFailure(ctx); failed != nil {
		return failed
	}
	if failed := s.neighborFocusGuard(ctx, source, target); failed != nil {
		return failed
	}
	focused, err := s.windows.FocusedWindow(ctx, target.PID)
	if err != nil {
		return focusedWindowFailure(err)
	}
	listed, err := s.windows.AllWindows(ctx)
	if err != nil {
		return failureOf(err)
	}
	neighbor, found := nearestListedWindow(focused, listed, s.selfPID, direction, options)
	if !found {
		return newFailure(ReasonNoNeighbor, "")
	}
	return s.raise(ctx, neighbor.PID, neighbor.Key)
}

func (s *Service) neighborFocusGuard(ctx context.Context, source Source, target Target) *Failure {
	if source == SourcePalette {
		return s.restorePaletteFocus(ctx, target)
	}
	return s.guardShortcutFocus(ctx, target)
}

func focusedWindowFailure(err error) *Failure {
	if failureOf(err).Reason == ReasonNoWindow {
		return newFailure(ReasonNoWindow, messageNoFocusedWindow)
	}
	return failureOf(err)
}

// nearestListedWindow picks the neighbor of the focused window among the listed windows, excluding
// Nevermind's own windows and the origin. The origin takes its z-index from the listing; a focused window
// missing from the listing ranks front-most.
func nearestListedWindow(focused platform.WindowRecord, listed []platform.WindowRecord, selfPID int, direction geometry.NeighborDirection, options geometry.NeighborOptions) (platform.WindowRecord, bool) {
	origin := neighborWindowOf(focused, 0)
	for _, record := range listed {
		if record.Key == focused.Key {
			origin = neighborWindowOf(record, record.ZIndex)
		}
	}
	candidates := []geometry.NeighborWindow{}
	byKey := map[string]platform.WindowRecord{}
	for _, record := range listed {
		if !isNeighborCandidate(record, selfPID, origin.ID) {
			continue
		}
		candidates = append(candidates, neighborWindowOf(record, record.ZIndex))
		byKey[record.Key] = record
	}
	key, found := geometry.NearestWindowInDirection(origin, candidates, direction, options)
	if !found {
		return platform.WindowRecord{}, false
	}
	return byKey[key], true
}

func isNeighborCandidate(record platform.WindowRecord, selfPID int, originKey string) bool {
	return record.PID != selfPID && record.Key != originKey && !record.Minimized && !record.FullScreen
}
