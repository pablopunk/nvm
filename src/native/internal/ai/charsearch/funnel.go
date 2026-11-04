package charsearch

import (
	"context"
	"fmt"
	"sort"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// The funnel's limits. A decision answers at most maxGroupQuestions or maxQuestions questions, and a choice question
// offers at most choicesPerQuestion characters plus "none of these".
const (
	maxGroupQuestions   = 16
	maxQuestions        = 16
	choicesPerQuestion  = 31
	recallPoolLimit     = maxQuestions * choicesPerQuestion
	categoryThreshold   = 0.3
	shortlistMinimum    = 0.02
	matchThreshold      = 0.5
	noneOfThese         = "none of these"
	choiceQuestionAsked = "Which option is the best match for the search text?"
)

// likelyGroups returns the groups that could hold a match for query, most likely first. A chosen category skips the
// question and stands for itself when the model asks about it; a category that is not asked about never matches.
func likelyGroups(ctx context.Context, decider Decider, query, category string, groups []string) ([]string, error) {
	if category != allCategory {
		if contains(groups, category) {
			return []string{category}, nil
		}
		return nil, nil
	}
	if len(groups) == 0 {
		return nil, nil
	}
	questions := make([]backend.DecisionQuestion, 0, len(groups))
	for i, group := range groups {
		questions = append(questions, backend.DecisionQuestion{
			ID:       fmt.Sprintf("group_%d", i),
			Type:     "predicate",
			Question: fmt.Sprintf(`Would an emoji or symbol from the "%s" category be a good result for this search text?`, group),
		})
	}
	result, err := decide(ctx, decider, query, questions)
	if err != nil {
		return nil, err
	}
	type scoredGroup struct {
		group       string
		probability float64
	}
	var scored []scoredGroup
	for i, group := range groups {
		answer, ok := result.Answers[fmt.Sprintf("group_%d", i)]
		if p := predicateProbability(answer, ok); p >= categoryThreshold {
			scored = append(scored, scoredGroup{group, p})
		}
	}
	sort.SliceStable(scored, func(a, b int) bool { return scored[a].probability > scored[b].probability })
	out := make([]string, 0, len(scored))
	for _, s := range scored {
		out = append(out, s.group)
	}
	return out, nil
}

// recallPool returns the characters of the groups in order, each label once, up to the pool limit.
func recallPool(groups []string, byGroup map[string][]Record) []Record {
	seen := map[string]bool{}
	var pool []Record
	for _, group := range groups {
		for _, r := range byGroup[group] {
			if seen[r.label()] {
				continue
			}
			seen[r.label()] = true
			pool = append(pool, r)
		}
	}
	if len(pool) > recallPoolLimit {
		pool = pool[:recallPoolLimit]
	}
	return pool
}

// shortlist asks which characters of the pool fit the text, one choice question per chunk of the pool, and keeps the
// best maxQuestions of the candidates the model ranks.
func shortlist(ctx context.Context, decider Decider, query string, pool []Record) ([]Record, error) {
	byLabel := make(map[string]Record, len(pool))
	for _, r := range pool {
		byLabel[r.label()] = r
	}
	var questions []backend.DecisionQuestion
	for start := 0; start < len(pool); start += choicesPerQuestion {
		chunk := pool[start:min(start+choicesPerQuestion, len(pool))]
		options := make([]string, 0, len(chunk)+1)
		for _, r := range chunk {
			options = append(options, r.label())
		}
		questions = append(questions, backend.DecisionQuestion{
			ID:       fmt.Sprintf("recall_%d", len(questions)),
			Type:     "choice",
			Question: choiceQuestionAsked,
			Options:  append(options, noneOfThese),
		})
	}
	result, err := decide(ctx, decider, query, questions)
	if err != nil {
		return nil, err
	}
	var ranked []choiceCandidate
	for i := range questions {
		answer, ok := result.Answers[fmt.Sprintf("recall_%d", i)]
		for _, candidate := range choiceCandidates(answer, ok) {
			if candidate.label != noneOfThese && candidate.probability >= shortlistMinimum {
				ranked = append(ranked, candidate)
			}
		}
	}
	sortCandidates(ranked)
	var out []Record
	for _, candidate := range firstCandidates(ranked, maxQuestions) {
		if r, ok := byLabel[candidate.label]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

// verify asks, for each shortlisted character, whether it matches the text, and keeps the matches whose probability
// reaches the threshold, best first.
func verify(ctx context.Context, decider Decider, query string, candidates []Record) ([]Record, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	questions := make([]backend.DecisionQuestion, 0, len(candidates))
	for i, r := range candidates {
		questions = append(questions, backend.DecisionQuestion{
			ID:       fmt.Sprintf("match_%d", i),
			Type:     "predicate",
			Question: fmt.Sprintf("Is %s a good match for the search text?", r.label()),
		})
	}
	result, err := decide(ctx, decider, query, questions)
	if err != nil {
		return nil, err
	}
	type scoredRecord struct {
		record      Record
		probability float64
	}
	var scored []scoredRecord
	for i, r := range candidates {
		answer, ok := result.Answers[fmt.Sprintf("match_%d", i)]
		if p := predicateProbability(answer, ok); p >= matchThreshold {
			scored = append(scored, scoredRecord{r, p})
		}
	}
	sort.SliceStable(scored, func(a, b int) bool { return scored[a].probability > scored[b].probability })
	out := make([]Record, 0, len(scored))
	for _, s := range scored {
		out = append(out, s.record)
	}
	return out, nil
}

// sortCandidates orders the candidates best first, ties by label, so one answer always ranks the same way.
func sortCandidates(candidates []choiceCandidate) {
	sort.SliceStable(candidates, func(a, b int) bool {
		if candidates[a].probability != candidates[b].probability {
			return candidates[a].probability > candidates[b].probability
		}
		return candidates[a].label < candidates[b].label
	})
}

// firstCandidates returns the first n candidates.
func firstCandidates(candidates []choiceCandidate, n int) []choiceCandidate {
	if len(candidates) > n {
		return candidates[:n]
	}
	return candidates
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
