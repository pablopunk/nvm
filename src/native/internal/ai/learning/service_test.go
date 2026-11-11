package learning

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestServiceDefersBusyExitAndDeduplicates(t *testing.T) {
	rules, _ := newRules(t)
	wakes := 0
	calls := 0
	svc := NewService(rules, func(context.Context, string) (string, error) {
		calls++
		return `{"proposals":[{"operation":"add","kind":"preference","text":"Prefer short labels"}]}`, nil
	}, func() (string, error) { return "one", nil }, func() { wakes++ })
	svc.Message("c", "user", "Prefer short labels")
	svc.Outcome("c", "success")
	svc.Exit("c", true)
	if wakes != 0 {
		t.Fatal("review started during active turn")
	}
	svc.Settled("c")
	if wakes != 1 {
		t.Fatal("settled exit not scheduled")
	}
	if err := svc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	svc.Exit("c", false)
	if wakes != 1 || calls != 1 {
		t.Fatal("unchanged trace reviewed again")
	}
	doc, err := svc.Snapshot()
	if err != nil || len(doc.Rules) != 1 {
		t.Fatalf("rules: %+v %v", doc, err)
	}
}

func TestServiceDiscardsReviewAfterDelete(t *testing.T) {
	rules, _ := newRules(t)
	if err := rules.ApplyAdds(0, []Rule{{ID: "one", Kind: "preference", Text: "Prefer clear labels"}}); err != nil {
		t.Fatal(err)
	}
	var svc *Service
	svc = NewService(rules, func(context.Context, string) (string, error) {
		doc, _ := svc.Snapshot()
		if err := svc.Delete(doc.Revision, "one", doc.Rules[0].Revision); err != nil {
			t.Fatal(err)
		}
		return `{"proposals":[{"operation":"add","kind":"preference","text":"Prefer clear labels"}]}`, nil
	}, func() (string, error) { return "two", nil }, nil)
	svc.Message("c", "user", "Use clear labels")
	svc.Outcome("c", "success")
	svc.Exit("c", false)
	if err := svc.Run(context.Background()); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale: %v", err)
	}
	doc, _ := svc.Snapshot()
	if len(doc.Rules) != 0 {
		t.Fatal("deleted rule returned")
	}
}

func TestServiceDeduplicatesHeldReviewAndQueuesNewerTrace(t *testing.T) {
	rules, _ := newRules(t)
	entered := make(chan struct{}, 2)
	release := make(chan struct{}, 2)
	done := make(chan error, 2)
	svc := NewService(rules, func(context.Context, string) (string, error) {
		entered <- struct{}{}
		<-release
		return `{"proposals":[]}`, nil
	}, func() (string, error) { return "new", nil }, nil)
	svc.Message("c", "user", "Prefer simple names")
	svc.Outcome("c", "success")
	svc.Exit("c", false)
	go func() { done <- svc.Run(context.Background()) }()
	<-entered
	svc.Message("c", "user", "Prefer short titles")
	svc.Exit("c", false)
	if err := svc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
		t.Fatal("concurrent review started for the same chat")
	default:
	}
	release <- struct{}{}
	if err := <-done; !errors.Is(err, ErrConflict) {
		t.Fatalf("stale: %v", err)
	}
	go func() { done <- svc.Run(context.Background()) }()
	<-entered
	release <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServiceBoundsBusyWaitingChats(t *testing.T) {
	rules, _ := newRules(t)
	svc := NewService(rules, nil, nil, nil)
	for i := 0; i < maxTraces+10; i++ {
		svc.Exit(fmt.Sprintf("c-%d", i), true)
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.tracked) != maxTraces || len(svc.waiting) != maxTraces || len(svc.pending) != 0 || len(svc.inflight) != 0 {
		t.Fatalf("unbounded tracking: %d %d %d %d", len(svc.tracked), len(svc.waiting), len(svc.pending), len(svc.inflight))
	}
}

func TestAutomaticReviewReachesLaterBuilderContext(t *testing.T) {
	rules, _ := newRules(t)
	scheduled := make(chan struct{}, 1)
	calls := 0
	svc := NewService(rules, func(_ context.Context, input string) (string, error) {
		calls++
		if !strings.Contains(input, "Prefer compact labels") || strings.Contains(input, "private-canary") {
			t.Fatal("review received unexpected capture")
		}
		return `{"proposals":[{"operation":"add","kind":"preference","text":"Prefer compact labels"}]}`, nil
	}, func() (string, error) { return "host-generated", nil }, func() {
		select {
		case scheduled <- struct{}{}:
		default:
		}
	})
	mailbox := NewMailbox(svc)
	defer mailbox.Close()
	mailbox.Capture("builder", []Message{{Role: "user", Text: "Prefer compact labels"}, {Role: "assistant", Text: "password private-canary"}}, []string{"success"})
	mailbox.Exit("builder", true)
	mailbox.Settled("builder")
	<-scheduled
	if err := svc.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	doc, err := svc.Snapshot()
	if err != nil || calls != 1 || !strings.Contains(Context(doc, ""), "Prefer compact labels") {
		t.Fatalf("context missing committed rule: %v", err)
	}
}

func TestCaptureFromTurnBeforeDeletionIsDiscarded(t *testing.T) {
	rules, _ := newRules(t)
	if err := rules.ApplyAdds(0, []Rule{{ID: "r", Kind: "preference", Text: "Prefer clear labels"}}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(rules, nil, nil, nil)
	epoch := svc.Epoch()
	doc, _ := svc.Snapshot()
	if err := svc.Delete(doc.Revision, "r", doc.Rules[0].Revision); err != nil {
		t.Fatal(err)
	}
	svc.CaptureEpoch("c", epoch, []Message{{Role: "user", Text: "Prefer clear labels"}}, []string{"success"})
	if _, ok := svc.traces.Snapshot("c"); ok {
		t.Fatal("pre-deletion turn was recaptured")
	}
}
