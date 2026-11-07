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
	//lint:ignore SA1012 Deliberately exercise the nil-context fallback.
	marked := WithCommandRun(nil)
	if _, ok := ConsumeCommandRun(marked); !ok {
		t.Fatal("nil input could not be marked")
	}
	//lint:ignore SA1012 Deliberately exercise the nil-context fallback.
	ctx, ok := ConsumeCommandRun(nil)
	if ok || ctx == nil {
		t.Fatal("nil input should yield an unmarked usable context")
	}
}

func TestCommandOriginMarkerConsumptionAndCompatibility(t *testing.T) {
	origin := CommandOrigin{extensionID: "builtin.sample", commandID: "run"}
	source := WithCommandRunOrigin(context.Background(), origin)
	consumed, gotOrigin, marked := ConsumeCommandOrigin(source)
	if !marked || gotOrigin != origin {
		t.Fatalf("got (%v, %#v)", marked, gotOrigin)
	}
	if _, _, marked := ConsumeCommandOrigin(consumed); marked {
		t.Fatal("origin marker consumed twice")
	}
	if _, gotOrigin, marked := ConsumeCommandOrigin(source); !marked || gotOrigin != origin {
		t.Fatal("source context was mutated")
	}
	boolConsumed, marked := ConsumeCommandRun(source)
	if !marked {
		t.Fatal("bool consumer did not consume origin marker")
	}
	if _, _, marked := ConsumeCommandOrigin(boolConsumed); marked {
		t.Fatal("bool consumer left origin visible")
	}
	legacy := WithCommandRun(context.Background())
	_, gotOrigin, marked = ConsumeCommandOrigin(legacy)
	if !marked || gotOrigin != (CommandOrigin{}) {
		t.Fatal("bool marker compatibility failed")
	}
}

func TestCommandOriginMarkerPreservesContextAndHandlesNil(t *testing.T) {
	origin := CommandOrigin{extensionID: "builtin.sample", commandID: "run"}
	base, cancel := context.WithCancel(context.WithValue(context.Background(), threadContextKey{}, "preserved"))
	marked := WithCommandRunOrigin(base, origin)
	cancel()
	consumed, got, ok := ConsumeCommandOrigin(marked)
	if !ok || got != origin || consumed.Err() != context.Canceled || consumed.Value(threadContextKey{}) != "preserved" {
		t.Fatal("context or origin lost")
	}
	//lint:ignore SA1012 Deliberately exercise the nil-context fallback.
	nilMarked := WithCommandRunOrigin(nil, origin)
	if _, got, ok := ConsumeCommandOrigin(nilMarked); !ok || got != origin {
		t.Fatal("nil source could not be marked")
	}
	//lint:ignore SA1012 Deliberately exercise the nil-context fallback.
	usable, got, ok := ConsumeCommandOrigin(nil)
	if usable == nil || ok || got != (CommandOrigin{}) {
		t.Fatal("nil source was not empty and usable")
	}
}
