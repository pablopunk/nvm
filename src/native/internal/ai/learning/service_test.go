package learning

import (
	"context"
	"errors"
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
