package safego

import (
	"context"
	"errors"
	"testing"
)

func TestGroupReturnsNilWhenEveryTaskSucceeds(t *testing.T) {
	g := NewGroup(WithReporter(newRecordingReporter("")))
	g.Go("a", func(context.Context) error { return nil })
	g.Go("b", func(context.Context) error { return nil })
	if err := waitWithWatchdog(t, g); err != nil {
		t.Fatalf("Wait = %v, want nil", err)
	}
}

func TestGroupCancelsSiblingsOnTheFirstFailure(t *testing.T) {
	sentinel := errors.New("disk gone")
	g := NewGroup(WithReporter(newRecordingReporter("")))
	g.Go("fail", func(context.Context) error { return sentinel })
	g.Go("waits", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err := waitWithWatchdog(t, g); !errors.Is(err, sentinel) {
		t.Fatalf("Wait = %v, want the first failure %v", err, sentinel)
	}
}

func TestGroupContainsPanicsAsErrors(t *testing.T) {
	g := NewGroup(WithReporter(newRecordingReporter("NVM-TEST4")))
	g.Go("panics", func(context.Context) error { panic("bad provider") })
	var panicked *PanicError
	if err := waitWithWatchdog(t, g); !errors.As(err, &panicked) || panicked.Ref != "NVM-TEST4" {
		t.Fatalf("Wait = %v, want a *PanicError with the reference", err)
	}
}
