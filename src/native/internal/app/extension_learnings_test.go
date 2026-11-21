package app

import (
	"bytes"
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/learning"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
	"github.com/pablopunk/nvm/native/internal/store"
)

// learningsHarness opens the real learning service over a fake file system, with one stored rule, and wires it to an
// App the way startBuilderLearning does.
func learningsHarness(t *testing.T, ask func(context.Context, string) (string, error)) (extensionLearnings, *learning.Service) {
	t.Helper()
	st, err := store.Open(fake.NewFS(), fake.NewClock(), testpath.Abs("learnings"), store.WithKeepBackups(0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	doc := store.Doc(st, "builder-learnings.json", 1, func() learning.Document { return learning.Document{} })
	rules, err := learning.OpenRules(doc, bytes.Repeat([]byte("k"), 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := rules.ApplyAdds(0, []learning.Rule{{ID: "one", Kind: "preference", Text: "Prefer clear labels", AppliesWhen: "naming"}}); err != nil {
		t.Fatal(err)
	}
	service := learning.NewService(rules, ask, func() (string, error) { return "new", nil }, nil)
	p := &App{}
	p.builderLearning.Store(&builderLearningRuntime{service: service})
	return extensionLearnings{p: p}, service
}

func TestLearningsPortListsRulesWithRevisions(t *testing.T) {
	l, _ := learningsHarness(t, nil)
	list, err := l.Rules()
	if err != nil || len(list.Rules) != 1 || list.Revision != 1 || list.Rules[0].Revision != 1 || list.Rules[0].AppliesWhen != "naming" {
		t.Fatalf("list = %+v, %v", list, err)
	}
}

func TestLearningsPortEditKeepsKindAndRefusesStaleEdit(t *testing.T) {
	l, _ := learningsHarness(t, nil)
	list, _ := l.Rules()
	outcome, err := l.Edit(list.Revision, "one", list.Rules[0].Revision, "Prefer short labels")
	if err != nil || outcome != api.LearningSaved {
		t.Fatalf("edit = %v, %v", outcome, err)
	}
	after, _ := l.Rules()
	if got := after.Rules[0]; got.Text != "Prefer short labels" || got.Kind != "preference" || got.AppliesWhen != "naming" || got.Revision != 2 {
		t.Fatalf("rule = %+v", got)
	}
	// A second editor still holding the first list must be refused, and nothing is overwritten.
	outcome, err = l.Edit(list.Revision, "one", list.Rules[0].Revision, "Overwrite")
	if err != nil || outcome != api.LearningStale {
		t.Fatalf("stale edit = %v, %v", outcome, err)
	}
	if again, _ := l.Rules(); again.Rules[0].Text != "Prefer short labels" {
		t.Fatalf("stale edit overwrote the rule: %+v", again.Rules[0])
	}
}

func TestLearningsPortRejectsInvalidText(t *testing.T) {
	l, _ := learningsHarness(t, nil)
	list, _ := l.Rules()
	for _, text := range []string{"   ", "see https://example.com/x", "my password is hunter2"} {
		outcome, err := l.Edit(list.Revision, "one", list.Rules[0].Revision, text)
		if err != nil || outcome != api.LearningInvalid {
			t.Fatalf("edit %q = %v, %v", text, outcome, err)
		}
	}
}

func TestLearningsPortDeleteIsGuardedByRevisions(t *testing.T) {
	l, _ := learningsHarness(t, nil)
	list, _ := l.Rules()
	if outcome, err := l.Delete(list.Revision+1, "one", list.Rules[0].Revision); err != nil || outcome != api.LearningStale {
		t.Fatalf("stale delete = %v, %v", outcome, err)
	}
	if outcome, err := l.Delete(list.Revision, "one", list.Rules[0].Revision); err != nil || outcome != api.LearningSaved {
		t.Fatalf("delete = %v, %v", outcome, err)
	}
	if after, _ := l.Rules(); len(after.Rules) != 0 {
		t.Fatalf("rule survived: %+v", after.Rules)
	}
}

func TestLearningsPortDeletedRuleIsNotRestoredByEarlierReview(t *testing.T) {
	var l extensionLearnings
	ask := func(context.Context, string) (string, error) {
		// The user deletes the rule while the review is still running.
		list, _ := l.Rules()
		if outcome, err := l.Delete(list.Revision, "one", list.Rules[0].Revision); err != nil || outcome != api.LearningSaved {
			t.Errorf("delete during review = %v, %v", outcome, err)
		}
		return `{"proposals":[{"operation":"add","kind":"preference","text":"Prefer clear labels"}]}`, nil
	}
	var service *learning.Service
	l, service = learningsHarness(t, ask)
	service.Message("chat", "user", "Use clear labels")
	service.Outcome("chat", "success")
	service.Exit("chat", false)
	_ = service.Run(context.Background())
	if after, _ := l.Rules(); len(after.Rules) != 0 {
		t.Fatalf("deleted rule came back: %+v", after.Rules)
	}
}

func TestLearningsPortEditDuringReviewDiscardsStaleReview(t *testing.T) {
	var l extensionLearnings
	ask := func(context.Context, string) (string, error) {
		list, _ := l.Rules()
		if outcome, err := l.Edit(list.Revision, "one", list.Rules[0].Revision, "Prefer short labels"); err != nil || outcome != api.LearningSaved {
			t.Errorf("edit during review = %v, %v", outcome, err)
		}
		return `{"proposals":[{"operation":"remove","id":"one"}]}`, nil
	}
	var service *learning.Service
	l, service = learningsHarness(t, ask)
	service.Message("chat", "user", "Use clear labels")
	service.Outcome("chat", "success")
	service.Exit("chat", false)
	_ = service.Run(context.Background())
	after, _ := l.Rules()
	if len(after.Rules) != 1 || after.Rules[0].Text != "Prefer short labels" {
		t.Fatalf("the edit was lost to an earlier review: %+v", after.Rules)
	}
}

func TestLearningsPortUnavailableBeforeStoreOpens(t *testing.T) {
	l := extensionLearnings{p: &App{}}
	if _, err := l.Rules(); err == nil {
		t.Fatal("list answered without a store")
	}
}
