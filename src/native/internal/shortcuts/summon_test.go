package shortcuts

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const (
	configured = "Alt+Shift+Space"
	replaced   = "Control+Alt+J"
	recovered  = "Control+Alt+K"
)

// harness is a registrar over the fake OS shortcuts and clock. Its status changes arrive on changes.
type harness struct {
	hotkeys *fake.Hotkeys
	clock   *fake.Clock
	summon  *Summon
	changes chan Status
}

func newHarness(t *testing.T, goos string) *harness {
	t.Helper()
	h := &harness{hotkeys: fake.NewHotkeys(), clock: fake.NewClock(), changes: make(chan Status, 64)}
	h.summon = NewSummon(SummonOptions{
		Hotkeys:  h.hotkeys,
		Clock:    h.clock,
		GOOS:     goos,
		Toggle:   func() {},
		OnChange: func(st Status) { h.changes <- st },
	})
	return h
}

// otherProgramHolds simulates another app that owns accelerator, as the OS would refuse it.
func (h *harness) otherProgramHolds(t *testing.T, accelerator string) {
	t.Helper()
	if err := h.hotkeys.Register(accelerator, func() {}); err != nil {
		t.Fatalf("setting up the other program: %v", err)
	}
}

// awaitStatus waits for the next status change that satisfies want and returns it. The wait ends on the
// change itself, with a watchdog that only fails the test.
func (h *harness) awaitStatus(t *testing.T, want func(Status) bool) Status {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case st := <-h.changes:
			if want(st) {
				return st
			}
		case <-deadline:
			t.Fatalf("no matching status; the registrar reports %+v", h.summon.Status())
		}
	}
}

func boundAccelerators(h *fake.Hotkeys) []string {
	return h.Registered()
}

func TestStartBindsTheConfiguredAccelerator(t *testing.T) {
	h := newHarness(t, "linux")
	st := h.summon.Start(configured)
	want := Status{Accelerator: configured, Registered: true}
	if st != want {
		t.Fatalf("Start = %+v, want %+v", st, want)
	}
	if got := boundAccelerators(h.hotkeys); !reflect.DeepEqual(got, []string{configured}) {
		t.Fatalf("bound = %v", got)
	}
}

func TestStartWithNoAcceleratorBindsNothing(t *testing.T) {
	h := newHarness(t, "linux")
	st := h.summon.Start("")
	if st != (Status{}) || len(boundAccelerators(h.hotkeys)) != 0 {
		t.Fatalf("Start(\"\") = %+v, bound %v", st, boundAccelerators(h.hotkeys))
	}
}

func TestConflictOnLinuxBindsTheFirstFreeRecoveryAccelerator(t *testing.T) {
	h := newHarness(t, "linux")
	h.otherProgramHolds(t, configured)
	st := h.summon.Start(configured)
	if st.Registered || st.Recovery != recovered || !st.Retrying {
		t.Fatalf("Start = %+v, want the recovery bound and the configured one retrying", st)
	}
	if got := boundAccelerators(h.hotkeys); !reflect.DeepEqual(got, []string{configured, recovered}) {
		t.Fatalf("bound = %v", got)
	}
}

func TestConflictOffLinuxHasNoRecoveryAccelerator(t *testing.T) {
	h := newHarness(t, "darwin")
	h.otherProgramHolds(t, configured)
	st := h.summon.Start(configured)
	if st.Registered || st.Recovery != "" {
		t.Fatalf("Start = %+v, want no recovery off Linux", st)
	}
}

func TestRetryPromotesTheConfiguredAcceleratorWhenItIsFreed(t *testing.T) {
	h := newHarness(t, "linux")
	h.otherProgramHolds(t, configured)
	h.summon.Start(configured)
	h.hotkeys.Unregister(configured)
	h.clock.AwaitTimers(1)
	h.clock.Advance(summonRetryEvery)
	st := h.awaitStatus(t, func(st Status) bool { return st.Registered })
	if st != (Status{Accelerator: configured, Registered: true}) {
		t.Fatalf("promoted status = %+v", st)
	}
	if got := boundAccelerators(h.hotkeys); !reflect.DeepEqual(got, []string{configured}) {
		t.Fatalf("bound after promotion = %v; the recovery must be released", got)
	}
}

func TestRetriesSettleAfterTheLastAttempt(t *testing.T) {
	h := newHarness(t, "linux")
	h.otherProgramHolds(t, configured)
	h.summon.Start(configured)
	for range summonRetries {
		h.clock.AwaitTimers(1)
		h.clock.Advance(summonRetryEvery)
	}
	st := h.awaitStatus(t, func(st Status) bool { return !st.Retrying })
	if st.Registered || st.Recovery != recovered {
		t.Fatalf("settled status = %+v, want the recovery and no retry", st)
	}
	if n := h.clock.PendingTimers(); n != 0 {
		t.Fatalf("%d timers still pending after the last attempt", n)
	}
}

func TestUnsupportedSessionDoesNotRetry(t *testing.T) {
	summon := NewSummon(SummonOptions{Hotkeys: unsupportedHotkeys{}, Clock: fake.NewClock(), GOOS: "linux", Toggle: func() {}})
	st := summon.Start(configured)
	if st.Registered || st.Retrying || st.Recovery != "" {
		t.Fatalf("Start on an unsupported session = %+v", st)
	}
}

func TestChangeMovesTheShortcut(t *testing.T) {
	h := newHarness(t, "linux")
	h.summon.Start(configured)
	if err := h.summon.Change(replaced); err != nil {
		t.Fatalf("Change: %v", err)
	}
	if got := boundAccelerators(h.hotkeys); !reflect.DeepEqual(got, []string{replaced}) {
		t.Fatalf("bound = %v", got)
	}
	if st := h.summon.Status(); st != (Status{Accelerator: replaced, Registered: true}) {
		t.Fatalf("status = %+v", st)
	}
}

func TestChangeToATakenAcceleratorRestoresThePrevious(t *testing.T) {
	h := newHarness(t, "linux")
	h.summon.Start(configured)
	h.otherProgramHolds(t, replaced)
	before := h.summon.Status()
	err := h.summon.Change(replaced)
	if !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("Change error = %v, want a conflict", err)
	}
	if got := boundAccelerators(h.hotkeys); !reflect.DeepEqual(got, []string{configured, replaced}) {
		t.Fatalf("bound = %v; the previous shortcut must be bound again", got)
	}
	if st := h.summon.Status(); st != before {
		t.Fatalf("status = %+v, want %+v unchanged", st, before)
	}
}

func TestChangeToNothingUnbindsTheShortcut(t *testing.T) {
	h := newHarness(t, "linux")
	h.summon.Start(configured)
	if err := h.summon.Change(""); err != nil {
		t.Fatalf("Change: %v", err)
	}
	if len(boundAccelerators(h.hotkeys)) != 0 || h.summon.Status() != (Status{}) {
		t.Fatalf("bound %v, status %+v", boundAccelerators(h.hotkeys), h.summon.Status())
	}
}

func TestChangeStopsTheRetryOfTheOldAccelerator(t *testing.T) {
	h := newHarness(t, "linux")
	h.otherProgramHolds(t, configured)
	h.summon.Start(configured)
	h.hotkeys.Unregister(configured)
	if err := h.summon.Change(replaced); err != nil {
		t.Fatalf("Change: %v", err)
	}
	h.clock.AwaitTimers(1)
	h.clock.Advance(summonRetryEvery)
	h.clock.Advance(summonRetryEvery)
	if got := boundAccelerators(h.hotkeys); !reflect.DeepEqual(got, []string{replaced}) {
		t.Fatalf("bound = %v; the old retry must not bind after a change", got)
	}
}

// unsupportedHotkeys is a session with no global shortcuts, as Wayland without the portal.
type unsupportedHotkeys struct{}

func (unsupportedHotkeys) Register(string, func()) error { return platform.ErrUnsupported }
func (unsupportedHotkeys) Unregister(string)             {}

func TestSummonNoticeExplainsEachState(t *testing.T) {
	label := func(accelerator string) string { return accelerator }
	tests := []struct {
		name string
		st   Status
		want SummonCopy
		show bool
	}{
		{"working", Status{Accelerator: configured, Registered: true}, SummonCopy{}, false},
		{"retrying", Status{Accelerator: configured, Retrying: true}, SummonCopy{}, false},
		{"unset", Status{}, SummonCopy{Title: "No shortcut set", Detail: "Set one to open Nevermind from anywhere.", ActionLabel: "Set shortcut"}, true},
		{"unavailable with recovery", Status{Accelerator: configured, Recovery: recovered}, SummonCopy{Title: configured + " is unavailable", Detail: "Use " + recovered + " to reopen Nevermind.", ActionLabel: "Change shortcut", Alert: true}, true},
		{"unavailable without recovery", Status{Accelerator: configured}, SummonCopy{Title: configured + " is unavailable", Detail: "Reopen Nevermind from your app launcher.", ActionLabel: "Change shortcut", Alert: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, show := SummonNotice(tt.st, label)
			if show != tt.show || got != tt.want {
				t.Fatalf("SummonNotice = %+v, %v; want %+v, %v", got, show, tt.want, tt.show)
			}
		})
	}
}
