package safego

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestRestartWaitsWithBackoffOnTheClock(t *testing.T) {
	clock := fake.NewClock()
	rep := newRecordingReporter("")
	attempts := 0
	done := make(chan error, 1)
	go func() {
		done <- Run("svc", func(context.Context) error {
			attempts++
			if attempts <= 2 {
				panic("transient")
			}
			return nil
		}, WithClock(clock), WithReporter(rep), WithBackoff(5, time.Minute))
	}()

	clock.AwaitTimers(1)
	clock.Advance(backoffBase - time.Millisecond)
	if n := attempts; n != 1 {
		t.Fatalf("restarted before the first backoff elapsed: attempts = %d", n)
	}
	clock.Advance(time.Millisecond)
	clock.AwaitTimers(1)
	clock.Advance(2 * backoffBase)

	if err := <-done; err != nil {
		t.Fatalf("Run = %v, want nil after the third attempt succeeds", err)
	}
	failures := rep.snapshot()
	if len(failures) != 2 {
		t.Fatalf("reported %d failures, want 2", len(failures))
	}
	for i, failure := range failures {
		if !failure.Panic || failure.Attempt != i+1 || failure.Disabled {
			t.Fatalf("failure %d = %+v, want a panic with attempt %d", i, failure, i+1)
		}
	}
}

func TestRepeatOffenderIsDisabledWithinWindow(t *testing.T) {
	clock := fake.NewClock()
	rep := newRecordingReporter("NVM-TEST3")
	done := make(chan error, 1)
	go func() {
		done <- Run("offender", func(context.Context) error {
			panic("always")
		}, WithClock(clock), WithReporter(rep), WithBackoff(2, time.Minute))
	}()

	clock.AwaitTimers(1)
	clock.Advance(backoffFor(1))
	clock.AwaitTimers(1)
	clock.Advance(backoffFor(2))

	err := <-done
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("Run = %v, want an error wrapping ErrDisabled", err)
	}
	var panicked *PanicError
	if !errors.As(err, &panicked) {
		t.Fatalf("Run = %v, want the last *PanicError in the chain", err)
	}
	failures := rep.snapshot()
	if len(failures) != 3 {
		t.Fatalf("reported %d failures, want 3", len(failures))
	}
	last := failures[len(failures)-1]
	if !last.Disabled || last.Attempt != 3 {
		t.Fatalf("last failure = %+v, want Disabled with attempt 3", last)
	}
	for _, failure := range failures[:2] {
		if failure.Disabled {
			t.Fatalf("an early failure was reported as Disabled: %+v", failure)
		}
	}
}

func TestFailuresOutsideTheWindowAreForgotten(t *testing.T) {
	clock := fake.NewClock()
	rep := newRecordingReporter("")
	gate := make(chan struct{})
	attempts := 0
	done := make(chan error, 1)
	go func() {
		done <- Run("slow-offender", func(context.Context) error {
			attempts++
			switch attempts {
			case 1:
				panic("first")
			case 2:
				<-gate
				panic("second")
			}
			return nil
		}, WithClock(clock), WithReporter(rep), WithBackoff(1, time.Second))
	}()

	clock.AwaitTimers(1)
	clock.Advance(backoffFor(1))
	clock.Advance(2 * time.Second)
	close(gate)

	clock.AwaitTimers(1)
	clock.Advance(backoffFor(1))
	if err := <-done; err != nil {
		t.Fatalf("Run = %v, want nil: the first failure is outside the window", err)
	}
	for _, failure := range rep.snapshot() {
		if failure.Disabled {
			t.Fatalf("disabled although the earlier failure expired: %+v", failure)
		}
	}
}
