package services

import (
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// firedRuns collects the runs the scheduler hands back to the host.
type firedRuns struct {
	runs chan api.Invocation
}

// scheduledHandler opens the services of one extension whose due runs are recorded in the returned collector.
func scheduledHandler(t *testing.T, r rig) (*Handler, *firedRuns) {
	t.Helper()
	fired := &firedRuns{runs: make(chan api.Invocation, 32)}
	ports := real.NewPorts()
	handler := New(Deps{
		Files:     ports.FS,
		Clock:     r.clock,
		DataRoot:  r.root,
		Keychain:  r.keychain,
		Network:   r.network,
		Clipboard: r.clipboard,
		Logs:      r.logs,
		RunScheduled: func(_ string, inv api.Invocation) {
			fired.runs <- inv
		},
	}, manifestFor("acme.schedule"))
	t.Cleanup(func() { _ = handler.Close() })
	return handler, fired
}

// awaitRun waits for one run to come back. The wait is only a guard against a run that never comes; the clock decides
// when it is due.
func (f *firedRuns) awaitRun(t *testing.T) api.Invocation {
	t.Helper()
	select {
	case inv := <-f.runs:
		return inv
	case <-time.After(5 * time.Second):
		t.Fatal("the due run never came back to the host")
	}
	return api.Invocation{}
}

// schedule asks the scheduler service to run ref under key at at.
func schedule(t *testing.T, h *Handler, key string, at time.Time, ref string) protocol.ServiceReply {
	t.Helper()
	return serve(t, h, protocol.ServiceScheduler, protocol.OpOnce, protocol.OnceArgs{Key: key, At: at, Ref: api.ActionRef{ID: ref}})
}

func TestDueRunComesBackOnceWithItsLaunchReasonAndNotBefore(t *testing.T) {
	r := newRig(t)
	h, fired := scheduledHandler(t, r)
	due := r.clock.Now().Add(10 * time.Second)
	requireOK(t, schedule(t, h, "timer", due, "finish"))
	r.clock.AwaitTimers(1)
	r.clock.Advance(9 * time.Second)
	if pending := pendingOf(t, h, "timer"); !pending.Pending || !pending.At.Equal(due) {
		t.Fatalf("pending after 9s = %+v, want the run due at %v", pending, due)
	}
	r.clock.Advance(time.Second)
	inv := fired.awaitRun(t)
	if inv.Ref.ID != "finish" || inv.Launch.Reason != "once" || inv.Launch.Event != "once" {
		t.Fatalf("fired run = %+v", inv)
	}
	if pending := pendingOf(t, h, "timer"); pending.Pending {
		t.Fatal("a fired run is still pending")
	}
}

func TestCancelDropsTheRunAndReportsWhetherOneWasPending(t *testing.T) {
	r := newRig(t)
	h, fired := scheduledHandler(t, r)
	requireOK(t, schedule(t, h, "timer", r.clock.Now().Add(time.Minute), "finish"))
	r.clock.AwaitTimers(1)
	if cancelled := cancelOf(t, h, "timer"); !cancelled {
		t.Fatal("cancel of a pending run reported none")
	}
	if cancelled := cancelOf(t, h, "timer"); cancelled {
		t.Fatal("a second cancel reported a run")
	}
	r.clock.Advance(time.Minute)
	select {
	case inv := <-fired.runs:
		t.Fatalf("a cancelled run fired: %+v", inv)
	default:
	}
}

func TestScheduleUnderAKeyReplacesThePendingRun(t *testing.T) {
	r := newRig(t)
	h, fired := scheduledHandler(t, r)
	requireOK(t, schedule(t, h, "timer", r.clock.Now().Add(10*time.Second), "first"))
	r.clock.AwaitTimers(1)
	requireOK(t, schedule(t, h, "timer", r.clock.Now().Add(20*time.Second), "second"))
	r.clock.AwaitTimers(2)
	r.clock.Advance(10 * time.Second)
	r.clock.Advance(10 * time.Second)
	if inv := fired.awaitRun(t); inv.Ref.ID != "second" {
		t.Fatalf("the replaced run ran: %+v", inv)
	}
}

func TestATimeAlreadyPastRunsAtOnce(t *testing.T) {
	r := newRig(t)
	h, fired := scheduledHandler(t, r)
	requireOK(t, schedule(t, h, "now", r.clock.Now().Add(-time.Second), "finish"))
	if inv := fired.awaitRun(t); inv.Ref.ID != "finish" {
		t.Fatalf("past run = %+v", inv)
	}
}

func TestPendingRunsAreLimitedPerExtension(t *testing.T) {
	r := newRig(t)
	h, _ := scheduledHandler(t, r)
	for i := range api.MaxScheduledRuns {
		requireOK(t, schedule(t, h, keyOf(i), r.clock.Now().Add(time.Hour), "finish"))
	}
	reply := schedule(t, h, "one-too-many", r.clock.Now().Add(time.Hour), "finish")
	if reply.Failure == nil {
		t.Fatal("a run past the pending limit was accepted")
	}
	requireOK(t, schedule(t, h, keyOf(0), r.clock.Now().Add(2*time.Hour), "finish"))
}

func TestBadKeysAndEmptyActionsAreRefused(t *testing.T) {
	r := newRig(t)
	h, _ := scheduledHandler(t, r)
	for name, reply := range map[string]protocol.ServiceReply{
		"empty key":    schedule(t, h, "", r.clock.Now(), "finish"),
		"long key":     schedule(t, h, strings.Repeat("k", api.ScheduleKeyLimit+1), r.clock.Now(), "finish"),
		"empty action": schedule(t, h, "timer", r.clock.Now(), ""),
	} {
		if reply.Failure == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestClosingTheHandlerDropsItsPendingRunsAndRefusesNewOnes(t *testing.T) {
	r := newRig(t)
	h, fired := scheduledHandler(t, r)
	requireOK(t, schedule(t, h, "timer", r.clock.Now().Add(time.Minute), "finish"))
	r.clock.AwaitTimers(1)
	if err := h.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	r.clock.Advance(time.Minute)
	select {
	case inv := <-fired.runs:
		t.Fatalf("a run of a closed extension fired: %+v", inv)
	default:
	}
	if reply := schedule(t, h, "later", r.clock.Now(), "finish"); reply.Failure == nil {
		t.Fatal("a closed extension scheduled a run")
	}
}

func TestSchedulingIsRateLimited(t *testing.T) {
	r := newRig(t)
	h, _ := scheduledHandler(t, r)
	for range schedulesPerMinute {
		requireOK(t, schedule(t, h, "timer", r.clock.Now().Add(time.Hour), "finish"))
	}
	if reply := schedule(t, h, "timer", r.clock.Now().Add(time.Hour), "finish"); reply.Failure == nil {
		t.Fatal("scheduling past the rate limit was accepted")
	}
}

// pendingOf asks the pending operation about key.
func pendingOf(t *testing.T, h *Handler, key string) protocol.PendingReply {
	t.Helper()
	var reply protocol.PendingReply
	raw := mustOK(t, serve(t, h, protocol.ServiceScheduler, protocol.OpPending, protocol.ScheduleKeyArgs{Key: key}))
	if err := jsonInto(raw, &reply); err != nil {
		t.Fatalf("pending reply: %v", err)
	}
	return reply
}

// cancelOf cancels key and reports whether a run was pending.
func cancelOf(t *testing.T, h *Handler, key string) bool {
	t.Helper()
	var cancelled bool
	raw := mustOK(t, serve(t, h, protocol.ServiceScheduler, protocol.OpCancel, protocol.ScheduleKeyArgs{Key: key}))
	if err := jsonInto(raw, &cancelled); err != nil {
		t.Fatalf("cancel reply: %v", err)
	}
	return cancelled
}

// requireOK fails the test when a scheduler call was refused.
func requireOK(t *testing.T, reply protocol.ServiceReply) {
	t.Helper()
	if reply.Failure != nil {
		t.Fatalf("scheduler call refused: %+v", *reply.Failure)
	}
}

// keyOf names the i-th test key.
func keyOf(i int) string {
	return "key-" + string(rune('a'+i))
}
