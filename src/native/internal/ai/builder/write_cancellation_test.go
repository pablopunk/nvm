package builder

import (
	"context"
	"testing"
)

type cancellingCheck struct {
	*fakeExtensions
	cancel context.CancelFunc
	checks int
}

func (e *cancellingCheck) Check(id string, files map[string][]byte) ([]string, error) {
	e.checks++
	e.cancel()
	return e.fakeExtensions.Check(id, files)
}

func TestCancelledBuilderWriteDoesNotInstall(t *testing.T) {
	for _, duringCheck := range []bool{false, true} {
		t.Run(map[bool]string{false: "already-cancelled", true: "during-check"}[duringCheck], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			extensions := &cancellingCheck{fakeExtensions: newFakeExtensions(), cancel: cancel}
			workshop := New(newFakeConversation(builderChat("builder")), extensions, synchronous)
			if !duringCheck {
				cancel()
			}
			if _, err := workshop.writeExtension(ctx, "builder", clockArgs(t)); err == nil {
				t.Fatal("cancelled write succeeded")
			}
			if len(extensions.installs) != 0 {
				t.Fatal("cancelled validation reached installation")
			}
			if !duringCheck && extensions.checks != 0 {
				t.Fatal("cancelled write reached candidate validation")
			}
			if duringCheck && extensions.checks != 1 {
				t.Fatal("test did not cancel at candidate validation boundary")
			}
		})
	}
}
