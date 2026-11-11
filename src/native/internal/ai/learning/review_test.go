package learning

import (
	"errors"
	"testing"
)

func TestReviewIsAtomicAndRejectsStaleAndDuplicateProposals(t *testing.T) {
	rules, _ := newRules(t)
	next := func() (string, error) { return "host-id", nil }
	if err := rules.ApplyReview(0, []Proposal{{Operation: "add", Kind: "preference", Text: "Prefer clear labels"}}, next); err != nil {
		t.Fatal(err)
	}
	state, _ := rules.Snapshot()
	bad := []Proposal{{Operation: "update", ID: "host-id", Kind: "preference", Text: "Prefer short labels"}, {Operation: "remove", ID: "host-id"}}
	if err := rules.ApplyReview(state.Revision, bad, next); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate: %v", err)
	}
	after, _ := rules.Snapshot()
	if after.Revision != state.Revision || after.Rules[0].Text != state.Rules[0].Text {
		t.Fatal("invalid review partially applied")
	}
	if err := rules.Delete(state.Revision, "host-id", state.Rules[0].Revision); err != nil {
		t.Fatal(err)
	}
	if err := rules.ApplyReview(state.Revision, []Proposal{{Operation: "add", Kind: "preference", Text: "Prefer clear labels"}}, next); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale review: %v", err)
	}
}
