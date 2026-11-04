package filesearch

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// recordingDecider answers with a fixed result and keeps the request it was sent.
type recordingDecider struct {
	result backend.DecisionResult
	err    error
	calls  []backend.DecisionRequest
}

func (d *recordingDecider) Decisions(_ context.Context, req backend.DecisionRequest) (backend.DecisionResult, error) {
	d.calls = append(d.calls, req)
	return d.result, d.err
}

func predicate(probability float64) backend.DecisionAnswer {
	return backend.DecisionAnswer{Type: "predicate", Predicate: probability >= MatchProbability, Probability: probability}
}

func TestJudgeAsksOnePredicatePerNameAndSendsNamesOnly(t *testing.T) {
	decider := &recordingDecider{result: backend.DecisionResult{Answers: map[string]backend.DecisionAnswer{}}}
	if _, err := Judge(context.Background(), decider, "holiday photos", []string{"beach.jpg", "tax.pdf"}); err != nil {
		t.Fatal(err)
	}
	if len(decider.calls) != 1 {
		t.Fatalf("decision calls = %d, want 1", len(decider.calls))
	}
	req := decider.calls[0]
	var input decisionInput
	if err := json.Unmarshal([]byte(req.Input), &input); err != nil {
		t.Fatalf("input is not JSON: %v", err)
	}
	if input.Query != "holiday photos" || !reflect.DeepEqual(input.Files, []decisionFileIn{{ID: "file_0", Name: "beach.jpg"}, {ID: "file_1", Name: "tax.pdf"}}) {
		t.Fatalf("input = %+v", input)
	}
	if len(req.Questions) != 2 || req.Questions[1].ID != "file_1" || req.Questions[1].Type != "predicate" ||
		req.Questions[1].Question != "Is file_1 relevant to the search query?" {
		t.Fatalf("questions = %+v", req.Questions)
	}
}

func TestJudgeReturnsTheCandidatesAtOrAboveTheThreshold(t *testing.T) {
	decider := &recordingDecider{result: backend.DecisionResult{Answers: map[string]backend.DecisionAnswer{
		"file_0": predicate(0.9),
		"file_1": predicate(0.1),
		"file_2": predicate(0.5),
	}}}
	got, err := Judge(context.Background(), decider, "q", []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("matches = %v, want [0 2]", got)
	}
}

func TestJudgeIgnoresMissingAndNonPredicateAnswers(t *testing.T) {
	decider := &recordingDecider{result: backend.DecisionResult{Answers: map[string]backend.DecisionAnswer{
		"file_0": {Type: "choice", Choice: "yes"},
		"file_1": {Type: "score", Score: 9},
	}}}
	got, err := Judge(context.Background(), decider, "q", []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("matches = %v, want none", got)
	}
}

func TestJudgeAsksNothingForNoCandidates(t *testing.T) {
	decider := &recordingDecider{}
	got, err := Judge(context.Background(), decider, "q", nil)
	if err != nil || got != nil || len(decider.calls) != 0 {
		t.Fatalf("got %v, %v with %d calls; want nothing and no call", got, err, len(decider.calls))
	}
}

func TestJudgePropagatesTheDecisionError(t *testing.T) {
	failure := &backend.Error{Kind: backend.KindInsufficientCredits, Message: "not enough credits"}
	decider := &recordingDecider{err: failure}
	if _, err := Judge(context.Background(), decider, "q", []string{"a"}); !errors.Is(err, failure) {
		t.Fatalf("err = %v, want the backend error", err)
	}
}

func TestDecisionInputNeverCarriesPaths(t *testing.T) {
	decider := &recordingDecider{}
	_, _ = Judge(context.Background(), decider, "q", []string{"notes.txt"})
	if strings.Contains(decider.calls[0].Input, "/") {
		t.Fatalf("input carries a path separator: %s", decider.calls[0].Input)
	}
}
