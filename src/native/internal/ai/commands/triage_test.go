package commands

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// TestTriageSummaryJoinsTheAnswersInOrder pins the indicator line: category, urgency, then the reply need.
func TestTriageSummaryJoinsTheAnswersInOrder(t *testing.T) {
	answers := map[string]backend.DecisionAnswer{
		"needs_reply": {Type: "predicate", Predicate: true, Probability: 0.9},
		"category":    {Type: "choice", Choice: "bug report"},
		"urgency":     {Type: "score", Score: 4},
	}
	if got, want := TriageSummary(answers), "bug report · urgency 4/5 · needs reply"; got != want {
		t.Fatalf("TriageSummary = %q, want %q", got, want)
	}
}

// TestTriageSummaryNamesNoReplyAndMissingAnswers covers a predicate that is false and answers that never came back.
func TestTriageSummaryNamesNoReplyAndMissingAnswers(t *testing.T) {
	answers := map[string]backend.DecisionAnswer{"needs_reply": {Type: "predicate"}}
	if got, want := TriageSummary(answers), "no reply needed"; got != want {
		t.Fatalf("TriageSummary = %q, want %q", got, want)
	}
	if got, want := TriageSummary(nil), "No triage result"; got != want {
		t.Fatalf("TriageSummary(nil) = %q, want %q", got, want)
	}
}
