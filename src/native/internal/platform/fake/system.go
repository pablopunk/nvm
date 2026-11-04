package fake

import (
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Operation names accepted by System.FailNext and recorded in SystemCall.Op.
const (
	OpSystemLock         = "LockScreen"
	OpSystemSleep        = "Sleep"
	OpSystemRestart      = "Restart"
	OpSystemShutDown     = "ShutDown"
	OpSystemLogOut       = "LogOut"
	OpSystemEmptyTrash   = "EmptyTrash"
	OpSystemOpenSettings = "OpenSettings"
)

// SystemCall is one recorded call on the system port. Arg is the settings pane and anchor of OpenSettings, joined
// with a slash, and empty for the other operations.
type SystemCall struct {
	Op  string
	Arg string
}

// System is a platform.System that records every session, power and settings call and performs none of them.
// Nothing here runs a program, so a test can drive destructive intents safely.
//
//	system := fake.NewSystem()
//	system.Calls() // every LockScreen, Restart... in order
type System struct {
	mu     sync.Mutex
	calls  []SystemCall
	events chan SystemCall
	faults faultSet
}

var _ platform.System = (*System)(nil)

// NewSystem returns a system port with no recorded calls.
func NewSystem() *System {
	return &System{events: make(chan SystemCall, 64)}
}

// Events delivers each recorded call as it happens, for a test that runs a command off the UI thread and waits for
// it. Calls beyond the buffer are still recorded in Calls, only not delivered.
func (s *System) Events() <-chan SystemCall {
	return s.events
}

// FailNext makes the next call of op return err. Use the Op constants of this file.
func (s *System) FailNext(op string, err error) {
	s.faults.failNext(op, err)
}

// Calls returns the recorded calls in order.
func (s *System) Calls() []SystemCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

// LockScreen records the call.
func (s *System) LockScreen() error { return s.record(OpSystemLock, "") }

// Sleep records the call.
func (s *System) Sleep() error { return s.record(OpSystemSleep, "") }

// Restart records the call.
func (s *System) Restart() error { return s.record(OpSystemRestart, "") }

// ShutDown records the call.
func (s *System) ShutDown() error { return s.record(OpSystemShutDown, "") }

// LogOut records the call.
func (s *System) LogOut() error { return s.record(OpSystemLogOut, "") }

// EmptyTrash records the call.
func (s *System) EmptyTrash() error { return s.record(OpSystemEmptyTrash, "") }

// OpenSettings records the call with its pane and anchor.
func (s *System) OpenSettings(pane, anchor string) error {
	arg := pane
	if anchor != "" {
		arg += "/" + anchor
	}
	return s.record(OpSystemOpenSettings, arg)
}

// record stores the call and returns the queued fault for op, if any.
func (s *System) record(op, arg string) error {
	if err := s.faults.take(op); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	call := SystemCall{Op: op, Arg: arg}
	s.calls = append(s.calls, call)
	select {
	case s.events <- call:
	default:
	}
	return nil
}
