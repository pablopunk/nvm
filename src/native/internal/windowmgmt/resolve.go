package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// writable is the window a command may change, with the display it sits on.
type writable struct {
	record   platform.WindowRecord
	window   geometry.WindowFrame
	display  geometry.Display
	displays []geometry.Display
}

// resolveWritable checks access, reads the window and rejects windows the command cannot change. A
// window that is full screen is only accepted by the full-screen toggle.
func (s *Service) resolveWritable(ctx context.Context, target Target, windowKey string, allowFullScreen bool) (writable, *Failure) {
	if failed := s.accessFailure(ctx); failed != nil {
		return writable{}, failed
	}
	record, failed := s.readTargetWindow(ctx, target, windowKey)
	if failed != nil {
		return writable{}, failed
	}
	if failed := rejectionFailure(record, target, allowFullScreen); failed != nil {
		return writable{}, failed
	}
	displays, err := s.windows.Displays(ctx)
	if err != nil {
		return writable{}, failureOf(err)
	}
	if len(displays) == 0 {
		return writable{}, newFailure(ReasonUnsupported, geometry.MessageNoDisplay)
	}
	converted := displaysFromPlatform(displays)
	window := windowFrameOf(record)
	display, err := geometry.PickDisplayForFrame(converted, window.Frame)
	if err != nil {
		return writable{}, newFailure(ReasonUnsupported, geometry.MessageNoDisplay)
	}
	return writable{record: record, window: window, display: display, displays: converted}, nil
}

// accessFailure is a denied failure unless the window system reports access. A window system that cannot answer is a
// failed helper, whose cause is kept for the log.
func (s *Service) accessFailure(ctx context.Context) *Failure {
	access, err := s.windows.Access(ctx)
	if err != nil {
		return failureOf(err)
	}
	if access != platform.WindowAccessAllowed {
		return newFailure(ReasonDenied, "")
	}
	return nil
}

func (s *Service) readTargetWindow(ctx context.Context, target Target, windowKey string) (platform.WindowRecord, *Failure) {
	if windowKey == "" {
		record, err := s.windows.FocusedWindow(ctx, target.PID)
		if err != nil {
			return platform.WindowRecord{}, failureOf(err)
		}
		return record, nil
	}
	records, err := s.windows.Windows(ctx, target.PID)
	if err != nil {
		return platform.WindowRecord{}, failureOf(err)
	}
	for _, record := range records {
		if record.Key == windowKey {
			return record, nil
		}
	}
	return platform.WindowRecord{}, newFailure(ReasonTargetLost, "")
}

func rejectionFailure(record platform.WindowRecord, target Target, allowFullScreen bool) *Failure {
	sameApp := record.PID == target.PID
	switch geometry.RejectTargetWindow(windowFrameOf(record), sameApp, allowFullScreen) {
	case geometry.RejectTargetChanged:
		return newFailure(ReasonTargetLost, geometry.MessageTargetWindowChanged)
	case geometry.RejectFullScreen:
		return newFailure(ReasonFullScreen, "")
	case geometry.RejectMinimized:
		return newFailure(ReasonMinimized, "")
	}
	return nil
}
