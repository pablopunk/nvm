package timers

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

var testNow = time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC)

// fakeScheduler holds the one-shot run the test schedules, as the host's scheduler would.
type fakeScheduler struct {
	api.Scheduler
	due     time.Time
	ref     api.ActionRef
	pending bool
}

func (s *fakeScheduler) Once(_ string, at time.Time, ref api.ActionRef) error {
	s.due, s.ref, s.pending = at, ref, true
	return nil
}

func (s *fakeScheduler) Cancel(string) (bool, error) {
	was := s.pending
	s.pending = false
	return was, nil
}

func (s *fakeScheduler) Pending(string) (time.Time, bool, error) {
	return s.due, s.pending, nil
}

// fakeSystem records the notifications the timer shows.
type fakeSystem struct {
	api.System
	notified []string
	refused  error
}

func (s *fakeSystem) Notify(title, body string) error {
	s.notified = append(s.notified, title+": "+body)
	return s.refused
}

// fakeViews counts the menu bar refreshes.
type fakeViews struct {
	api.Views
	refreshes *int
}

func (v fakeViews) RefreshMenuBar() { *v.refreshes++ }

// fakeContext is the context the built-in runs with, over one fake scheduler, system and clock.
type fakeContext struct {
	api.Context
	scheduler *fakeScheduler
	system    *fakeSystem
	refreshes *int
	now       time.Time
}

func newContext() fakeContext {
	return fakeContext{scheduler: &fakeScheduler{}, system: &fakeSystem{}, refreshes: new(int), now: testNow}
}

func (c fakeContext) Scheduler() api.Scheduler { return c.scheduler }
func (c fakeContext) System() api.System       { return c.system }
func (c fakeContext) Views() api.Views         { return fakeViews{refreshes: c.refreshes} }
func (c fakeContext) Clock() api.Clock         { return fakeClock{now: c.now} }

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

func TestTimerQueriesReadAsDurations(t *testing.T) {
	cases := map[string]time.Duration{
		"timer 5m":       5 * time.Minute,
		"timer 5":        5 * time.Minute,
		"countdown 90s":  90 * time.Second,
		"timer 1.5 min":  90 * time.Second,
		"timer 2h":       2 * time.Hour,
		"  Timer 10 sec": 10 * time.Second,
	}
	for query, want := range cases {
		if got, ok := parseTimer(query); !ok || got != want {
			t.Fatalf("parseTimer(%q) = %v, %v; want %v", query, got, ok, want)
		}
	}
	for _, query := range []string{"5m", "timer", "timer soon", "timers 5m"} {
		if _, ok := parseTimer(query); ok {
			t.Fatalf("parseTimer(%q) accepted a query that is not a timer", query)
		}
	}
}

func TestRootRowOffersToStartAnAnsweredDurationAboveTheCommands(t *testing.T) {
	rows, err := New().SearchItems(newContext(), "timer 5m")
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %d, %v", len(rows), err)
	}
	row := rows[0]
	if row.Title != "Start timer: 5m0s" || row.Section != "Timer" || row.Score != scoreTimer || row.Primary == nil || row.Primary.Title != "Start" {
		t.Fatalf("row = %+v", row)
	}
	if zero, _ := New().SearchItems(newContext(), "timer 0"); len(zero) != 0 {
		t.Fatal("a zero timer was offered")
	}
}

func TestStartRowSchedulesTheFinishAndTheMenuBarShowsTheCountdown(t *testing.T) {
	ctx := newContext()
	rows, _ := New().SearchItems(ctx, "timer 5m")
	if _, err := New().RunAction(ctx, api.Invocation{Ref: *rows[0].Primary.Ref}); err != nil {
		t.Fatal(err)
	}
	if !ctx.scheduler.pending || !ctx.scheduler.due.Equal(testNow.Add(5*time.Minute)) {
		t.Fatalf("scheduled = %+v", ctx.scheduler)
	}
	if ctx.scheduler.ref.ID != refFinish || ctx.scheduler.ref.Args[argDuration] != "5m0s" {
		t.Fatalf("finish ref = %+v", ctx.scheduler.ref)
	}
	items, _ := New().MenuBars(ctx)
	if len(items) != 1 || items[0].Title != "5:00" {
		t.Fatalf("menu bar = %+v", items)
	}
	ctx.now = testNow.Add(61 * time.Second)
	items, _ = New().MenuBars(ctx)
	if len(items) != 1 || items[0].Title != "3:59" {
		t.Fatalf("menu bar after 61s = %+v", items)
	}
}

func TestMenuBarIsEmptyWhileNoTimerRunsAndWhenItsTimeIsUp(t *testing.T) {
	ctx := newContext()
	if items, err := New().MenuBars(ctx); err != nil || len(items) != 0 {
		t.Fatalf("idle menu bar = %+v, %v", items, err)
	}
	ctx.scheduler.due, ctx.scheduler.pending = testNow, true
	if items, _ := New().MenuBars(ctx); len(items) != 0 {
		t.Fatalf("a timer that is up still shows: %+v", items)
	}
}

func TestStartTimerCommandSetsTheQueryAndCancelTimerIsListedOnlyWhileOneRuns(t *testing.T) {
	ctx := newContext()
	cmds, err := New().Commands(ctx)
	if err != nil || len(cmds) != 1 || cmds[0].Title != "Start Timer" || cmds[0].Placement[0] != api.PlacementSearch {
		t.Fatalf("idle commands = %+v, %v", cmds, err)
	}
	result, err := New().RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: startCommandID}})
	if err != nil || result.Next == nil || result.Next.Kind != api.ActionSetQuery || result.Next.Query != "timer 5m" {
		t.Fatalf("Start Timer result = %+v, %v", result, err)
	}
	ctx.scheduler.pending = true
	cmds, _ = New().Commands(ctx)
	if len(cmds) != 2 || cmds[1].Title != "Cancel Timer" || !cmds[1].DismissAfterRun {
		t.Fatalf("running commands = %+v", cmds)
	}
}

func TestCancelTimerDropsTheRunAndClearsTheMenuBar(t *testing.T) {
	ctx := newContext()
	ctx.scheduler.pending = true
	if _, err := New().RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: cancelCommandID}}); err != nil {
		t.Fatal(err)
	}
	if ctx.scheduler.pending || *ctx.refreshes != 1 {
		t.Fatalf("after cancel pending = %v, refreshes = %d", ctx.scheduler.pending, *ctx.refreshes)
	}
}

func TestFinishNotifiesThatTheTimerIsUpAndRefreshesTheMenuBar(t *testing.T) {
	ctx := newContext()
	ref := api.ActionRef{ID: refFinish, Args: map[string]string{argDuration: "5m0s"}}
	if _, err := New().RunAction(ctx, api.Invocation{Ref: ref}); err != nil {
		t.Fatal(err)
	}
	if len(ctx.system.notified) != 1 || ctx.system.notified[0] != "Timer finished: 5m0s is up" || *ctx.refreshes != 1 {
		t.Fatalf("notified = %v, refreshes = %d", ctx.system.notified, *ctx.refreshes)
	}
}

func TestANotificationTheSystemRefusesDoesNotFailTheFinish(t *testing.T) {
	ctx := newContext()
	ctx.system.refused = errors.New("notifications are off")
	ref := api.ActionRef{ID: refFinish, Args: map[string]string{argDuration: "1m0s"}}
	if _, err := New().RunAction(ctx, api.Invocation{Ref: ref}); err != nil {
		t.Fatalf("finish failed on a refused notification: %v", err)
	}
}

func TestMenuBarTicksEverySecondAndTheBuiltInNeedsTheSystemDeclaration(t *testing.T) {
	triggers := New().MenuBarTriggers()
	if len(triggers) != 1 || triggers[0].Type != api.TriggerInterval || triggers[0].Every != "1s" {
		t.Fatalf("triggers = %+v", triggers)
	}
	if caps := New().Manifest().Capabilities; len(caps) != 1 || caps[0] != api.CapabilitySystem {
		t.Fatalf("capabilities = %v", caps)
	}
}

func TestUnknownActionIsRefused(t *testing.T) {
	if _, err := New().RunAction(newContext(), api.Invocation{Ref: api.ActionRef{ID: "nope"}}); !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("unknown action error = %v", err)
	}
}
