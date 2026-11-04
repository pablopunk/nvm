package charsearch

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// DecisionTimeout is how long one decision call may take, as the 1.x picker's decision calls do.
const DecisionTimeout = 8 * time.Second

// DecisionBudget is how long the whole search may take: up to three decision calls and the typing pause. The
// coordinator cuts the provider off at this budget.
const DecisionBudget = 30 * time.Second

// Decider answers decision requests. The backend client satisfies it.
type Decider interface {
	Decisions(ctx context.Context, req backend.DecisionRequest) (backend.DecisionResult, error)
}

// askFunc sends one batch of questions about the search text and returns the answers.
type askFunc func(questions []backend.DecisionQuestion) (backend.DecisionResult, error)

// askModel is the askFunc of one search: each batch is counted against the rate limit, then sent with the decision
// timeout, as 1.x's ai.decide does.
func askModel(ctx context.Context, decider Decider, window *callWindow, query string) askFunc {
	return func(questions []backend.DecisionQuestion) (backend.DecisionResult, error) {
		if !window.allow() {
			return backend.DecisionResult{}, ErrRateLimited
		}
		ctx, cancel := context.WithTimeout(ctx, DecisionTimeout)
		defer cancel()
		return decider.Decisions(ctx, backend.DecisionRequest{Input: searchInput(query), Questions: questions})
	}
}

// searchInput is the text the model reads: the search text, quoted as JSON so any character survives.
func searchInput(query string) string {
	quoted, _ := json.Marshal(query)
	return "Search text: " + string(quoted)
}

// predicateProbability is the probability of a yes answer, or 0 when the answer is missing or not a predicate.
func predicateProbability(answer backend.DecisionAnswer, ok bool) float64 {
	if !ok || answer.Type != "predicate" {
		return 0
	}
	return answer.Probability
}

// choiceCandidate is one option a choice answer ranks, with its probability.
type choiceCandidate struct {
	label       string
	probability float64
}

// choiceCandidates returns the options a choice answer ranks. The model's probabilities are used when it sent them,
// and otherwise the chosen option counts with the answer's confidence, or the match threshold when it sent none.
func choiceCandidates(answer backend.DecisionAnswer, ok bool) []choiceCandidate {
	if !ok || answer.Type != "choice" {
		return nil
	}
	if answer.Probabilities != nil {
		out := make([]choiceCandidate, 0, len(answer.Probabilities))
		for label, probability := range answer.Probabilities {
			out = append(out, choiceCandidate{label: label, probability: probability})
		}
		return out
	}
	confidence := matchThreshold
	if answer.Confidence != nil {
		confidence = *answer.Confidence
	}
	return []choiceCandidate{{label: answer.Choice, probability: confidence}}
}

// normalizeQuery is the text the funnel answers: trimmed, lower case and capped, so repeats of one search share a cache entry.
func normalizeQuery(text string) string {
	runes := []rune(strings.ToLower(strings.TrimSpace(text)))
	if len(runes) > maxQueryLength {
		runes = runes[:maxQueryLength]
	}
	return string(runes)
}
