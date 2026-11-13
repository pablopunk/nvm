package safego

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestRunReturnsNilWhenFunctionSucceeds(t *testing.T) {
	rep := newRecordingReporter("")
	if err := Run("ok", func(context.Context) error { return nil }, WithReporter(rep)); err != nil {
		t.Fatalf("Run = %v, want nil", err)
	}
	if n := len(rep.snapshot()); n != 0 {
		t.Fatalf("reported %d failures for a success", n)
	}
}

func TestRunReturnsTheFunctionsError(t *testing.T) {
	want := errors.New("provider unavailable")
	rep := newRecordingReporter("")
	err := Run("provider", func(context.Context) error { return want }, WithReporter(rep))
	if !errors.Is(err, want) {
		t.Fatalf("Run = %v, want %v", err, want)
	}
	failures := rep.snapshot()
	if len(failures) != 1 || failures[0].Panic || failures[0].Name != "provider" {
		t.Fatalf("failures = %+v, want one non-panic failure named provider", failures)
	}
}

func TestPanicIsRecoveredReportedWithAReference(t *testing.T) {
	rep := newRecordingReporter("NVM-TEST1")
	err := Run("boom", func(context.Context) error { panic("provider exploded") }, WithReporter(rep))

	var panicked *PanicError
	if !errors.As(err, &panicked) {
		t.Fatalf("Run = %v, want *PanicError", err)
	}
	if panicked.Ref != "NVM-TEST1" {
		t.Fatalf("PanicError.Ref = %q, want the reporter's reference", panicked.Ref)
	}
	if !strings.Contains(panicked.Stack, "TestPanicIsRecoveredReportedWithAReference") {
		t.Fatalf("stack does not name the panicking function:\n%s", panicked.Stack)
	}
	failures := rep.snapshot()
	if len(failures) != 1 || !failures[0].Panic || failures[0].Stack == "" {
		t.Fatalf("failures = %+v, want one panic failure with a stack", failures)
	}
}

func TestGoRecoversPanicAndReportsIt(t *testing.T) {
	rep := newRecordingReporter("NVM-TEST2")
	Go("goroutine.boom", func(context.Context) { panic("nope") }, WithReporter(rep))
	select {
	case failure := <-rep.signal:
		if !failure.Panic || failure.Name != "goroutine.boom" {
			t.Fatalf("failure = %+v, want a panic named goroutine.boom", failure)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the panic was not reported")
	}
}

func TestTimeoutCancelsTheAttemptContext(t *testing.T) {
	clock := fake.NewClock()
	rep := newRecordingReporter("")
	cancelled := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run("slow", func(ctx context.Context) error {
			<-ctx.Done()
			close(cancelled)
			return ctx.Err()
		}, WithClock(clock), WithReporter(rep), WithTimeout(time.Second))
	}()

	clock.AwaitTimers(1)
	clock.Advance(time.Second)
	err := <-done

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run = %v, want a timeout wrapping context.DeadlineExceeded", err)
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("the attempt context was not cancelled")
	}
	if pending := clock.PendingTimers(); pending != 0 {
		t.Fatalf("%d timers still pending after the call returned", pending)
	}
	if failures := rep.snapshot(); len(failures) != 1 || !errors.Is(failures[0].Err, context.DeadlineExceeded) {
		t.Fatalf("failures = %+v, want one timeout failure", failures)
	}
}

func TestParentCancellationEndsTheCallWithoutReporting(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	rep := newRecordingReporter("")
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run("waits", func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}, WithContext(parent), WithReporter(rep))
	}()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run = %v, want context.Canceled", err)
	}
	if n := len(rep.snapshot()); n != 0 {
		t.Fatalf("reported %d failures for a shutdown", n)
	}
}

func TestTimeoutWithoutClockPanicsAtTheCallSite(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("WithTimeout without a clock did not panic")
		}
	}()
	_ = Run("no-clock", func(context.Context) error { return nil }, WithTimeout(time.Second))
}
