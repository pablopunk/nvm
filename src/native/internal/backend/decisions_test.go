package backend

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// decisionAnswersJSON is a decision response with one answer of each type.
const decisionAnswersJSON = `{"id":"dec_1","model":"fixture-decider","answers":{` +
	`"sunny":{"type":"predicate","value":true,"probability":0.9},` +
	`"color":{"type":"choice","value":"blue"},` +
	`"score":{"type":"score","value":4}},` +
	`"usage":{"inputTokens":120,"outputTokens":7}}`

// sampleDecisionRequest asks one question of each type.
func sampleDecisionRequest(key string) DecisionRequest {
	minLevel, maxLevel := 1, 5
	return DecisionRequest{
		Input:          "Is it sunny?",
		IdempotencyKey: key,
		Questions: []DecisionQuestion{
			{ID: "sunny", Type: "predicate", Question: "Is it sunny?"},
			{ID: "color", Type: "choice", Question: "Pick one", Options: []string{"red", "blue"}},
			{ID: "score", Type: "score", Question: "Rate it", Min: &minLevel, Max: &maxLevel},
		},
	}
}

// TestDecisionsSendsTheKeyAndDecodesEveryAnswerType checks the request carries the caller's idempotency key in
// its header rather than its body, and every answer type decodes into its typed field.
func TestDecisionsSendsTheKeyAndDecodesEveryAnswerType(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, decisionAnswersJSON))
	result, err := h.client.Decisions(context.Background(), sampleDecisionRequest("decision-key-1"))
	if err != nil {
		t.Fatalf("Decisions: %v", err)
	}
	request := h.http.Requests()[0]
	if got := request.Header.Get("Idempotency-Key"); got != "decision-key-1" {
		t.Fatalf("Idempotency-Key = %q, want the caller's key", got)
	}
	if strings.Contains(string(request.Body), "decision-key-1") {
		t.Fatalf("the idempotency key leaked into the body: %s", request.Body)
	}
	if !strings.Contains(string(request.Body), `"images":[]`) {
		t.Fatalf("body %s should send an empty images array, not null", request.Body)
	}
	if result.ID != "dec_1" || result.Model != "fixture-decider" || result.InputTokens != 120 || result.OutputTokens != 7 {
		t.Fatalf("result = %+v, want the decision id, model and usage", result)
	}
	predicate := result.Answers["sunny"]
	if predicate.Type != "predicate" || !predicate.Predicate || predicate.Probability != 0.9 {
		t.Fatalf("predicate answer = %+v", predicate)
	}
	if choice := result.Answers["color"]; choice.Choice != "blue" {
		t.Fatalf("choice answer = %+v", choice)
	}
	if score := result.Answers["score"]; score.Score != 4 {
		t.Fatalf("score answer = %+v", score)
	}
}

// TestDecisionsRejectsResponsesItCannotTrust checks every shape rule the desktop app enforces before using answers.
func TestDecisionsRejectsResponsesItCannotTrust(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing usage", body: `{"id":"d","model":"m","answers":{}}`},
		{name: "missing id", body: `{"model":"m","answers":{},"usage":{}}`},
		{name: "predicate without probability", body: `{"id":"d","model":"m","answers":{"a":{"type":"predicate","value":true}},"usage":{}}`},
		{name: "choice with a number", body: `{"id":"d","model":"m","answers":{"a":{"type":"choice","value":3}},"usage":{}}`},
		{name: "score with text", body: `{"id":"d","model":"m","answers":{"a":{"type":"score","value":"4"}},"usage":{}}`},
		{name: "unknown answer type", body: `{"id":"d","model":"m","answers":{"a":{"type":"essay","value":"x"}},"usage":{}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.http.Script(jsonBody(http.StatusOK, tt.body))
			_, err := h.client.Decisions(context.Background(), sampleDecisionRequest("k"))
			if !IsKind(err, KindBadResponse) {
				t.Fatalf("Decisions error = %v, want KindBadResponse", err)
			}
		})
	}
}

// TestDecisionsGeneratesAKeyWhenNoneIsGiven checks a caller without a key still sends one, so the backend can
// dedupe the request on a later retry by the client.
func TestDecisionsGeneratesAKeyWhenNoneIsGiven(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, decisionAnswersJSON))
	if _, err := h.client.Decisions(context.Background(), sampleDecisionRequest("")); err != nil {
		t.Fatalf("Decisions: %v", err)
	}
	if got := h.http.Requests()[0].Header.Get("Idempotency-Key"); got == "" {
		t.Fatal("no Idempotency-Key was sent")
	}
}

// TestDecisionsRejectsAnOversizedKeyLocally checks a key the backend would refuse is refused before any request.
func TestDecisionsRejectsAnOversizedKeyLocally(t *testing.T) {
	h := newHarness(t)
	_, err := h.client.Decisions(context.Background(), sampleDecisionRequest(strings.Repeat("k", 201)))
	if !IsKind(err, KindRejected) {
		t.Fatalf("Decisions error = %v, want KindRejected", err)
	}
	if n := len(h.http.Requests()); n != 0 {
		t.Fatalf("made %d requests for an invalid key, want none", n)
	}
}

// TestDecisionsIsNeverRetried checks a POST that fails in transit is not sent again by the client; a caller that
// wants another attempt reuses the key.
func TestDecisionsIsNeverRetried(t *testing.T) {
	h := newHarness(t)
	h.http.Script(fake.Response{Err: errors.New("connection reset by peer")}, jsonBody(http.StatusOK, decisionAnswersJSON))
	_, err := h.client.Decisions(context.Background(), sampleDecisionRequest("k"))
	if !IsKind(err, KindOffline) {
		t.Fatalf("Decisions error = %v, want KindOffline", err)
	}
	if n := len(h.http.Requests()); n != 1 {
		t.Fatalf("made %d requests, want 1", n)
	}
}
