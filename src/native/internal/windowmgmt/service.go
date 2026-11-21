package windowmgmt

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// Deps are the ports the engine uses. Windows is nil on systems without the window management
// capability, and the engine then reports every command as unsupported.
type Deps struct {
	Windows platform.WindowManager
	Focus   FocusHost
	Clock   platform.Clock
	// SelfPID is Nevermind's own process. Commands never act on it.
	SelfPID int
	// NewHandleID makes window handles; nil uses random hex.
	NewHandleID func() string
	// Log receives failed commands with their cause. Nil uses the process logger.
	Log *logx.Logger
}

// Service runs window commands. Each command returns a Result when the window reached its frame (exact
// or partial) and a *Failure otherwise. Callers run commands off the UI thread: they block on the window
// system and wait for their turn on the target process.
type Service struct {
	windows platform.WindowManager
	focus   FocusHost
	selfPID int
	queue   *pidQueue
	history *historyStore
	handles *handleRegistry
	log     *logx.Logger
}

// Result is a command that changed a window. Outcome is geometry.Applied when the window reached the
// requested frame and geometry.Partial when it did not, with Message explaining why.
type Result struct {
	Outcome geometry.Outcome
	Changed bool
	Frame   geometry.Rect
	Message string
}

// NewService builds the engine over its ports.
func NewService(deps Deps) *Service {
	return &Service{
		windows: deps.Windows,
		focus:   deps.Focus,
		selfPID: deps.SelfPID,
		queue:   newPIDQueue(),
		history: newHistoryStore(),
		handles: newHandleRegistry(deps.Clock, deps.NewHandleID),
		log:     deps.Log,
	}
}

// Available reports whether the system can manage windows at all.
func (s *Service) Available() bool {
	return s.windows != nil
}

// Apply runs a layout on the focused window of the invocation's target app.
func (s *Service) Apply(ctx context.Context, layout geometry.Layout, options geometry.Options, invocation Invocation) (Result, error) {
	result, err := s.applyInvocation(ctx, layout, options, invocation)
	s.reportFailure("apply", err)
	return result, err
}

func (s *Service) applyInvocation(ctx context.Context, layout geometry.Layout, options geometry.Options, invocation Invocation) (Result, error) {
	if failed := validateRequest(layout, options); failed != nil {
		return Result{}, failed
	}
	if !s.Available() {
		return Result{}, newFailure(ReasonUnsupported, "")
	}
	if invocation.Target == nil {
		return Result{}, newFailure(ReasonNoTarget, noTargetMessage(invocation.Source))
	}
	if invocation.Target.PID == s.selfPID {
		return Result{}, newFailure(ReasonNoTarget, messageSelfTarget)
	}
	return s.runRequest(ctx, request{
		layout:  layout,
		options: options,
		target:  *invocation.Target,
		source:  invocation.Source,
	})
}

func validateRequest(layout geometry.Layout, options geometry.Options) *Failure {
	if err := geometry.DescribeInvalidRequest(layout, options); err != nil {
		return newFailure(ReasonInvalidSpec, err.Error())
	}
	return nil
}

func noTargetMessage(source Source) string {
	if source == SourcePalette {
		return messageNoTargetPalette
	}
	return messageNoTargetShortcut
}
