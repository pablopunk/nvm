package manager

import (
	"context"
	"testing"
)

func TestCommandRunMarkerSurvivesDerivedContexts(t *testing.T) {
	base := WithCommandRun(context.WithValue(context.Background(), threadContextKey{}, "preserved"))
	cancelled, cancel := context.WithCancel(base)
	defer cancel()
	if cancelled.Value(threadContextKey{}) != "preserved" {
		t.Fatal("context value lost")
	}
	if _, marked := ConsumeCommandRun(cancelled); !marked {
		t.Fatal("cancel-derived context lost marker")
	}
	timed, stop := context.WithTimeout(base, 0)
	defer stop()
	if timed.Err() != context.DeadlineExceeded {
		t.Fatal("expected timed out context")
	}
	if _, marked := ConsumeCommandRun(timed); !marked {
		t.Fatal("timeout-derived context lost marker")
	}
}

func TestConsumeCommandRunShadowsWithoutMutatingSource(t *testing.T) {
	source := WithCommandRun(context.Background())
	consumed, marked := ConsumeCommandRun(source)
	if !marked {
		t.Fatal("marker not consumed")
	}
	if _, marked := ConsumeCommandRun(consumed); marked {
		t.Fatal("consumed marker remained visible")
	}
	if _, marked := ConsumeCommandRun(source); !marked {
		t.Fatal("source context was mutated")
	}
}

func TestCommandRunMarkerNilSafe(t *testing.T) {
	marked := WithCommandRun(nil)
	if _, ok := ConsumeCommandRun(marked); !ok {
		t.Fatal("nil input could not be marked")
	}
	ctx, ok := ConsumeCommandRun(nil)
	if ok || ctx == nil {
		t.Fatal("nil input should yield an unmarked usable context")
	}
}
