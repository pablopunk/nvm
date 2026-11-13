package charsearch

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// fakeModel answers the decision questions the way a model would, from the likelihoods the test sets: a group by its
// name, a character by its glyph. It records every call, so the tests can check how many questions each step asked.
type fakeModel struct {
	groups  map[string]float64
	glyphs  map[string]float64
	calls   [][]backend.DecisionQuestion
	inputs  []string
	failure error
}

func (m *fakeModel) Decisions(_ context.Context, req backend.DecisionRequest) (backend.DecisionResult, error) {
	m.calls = append(m.calls, req.Questions)
	m.inputs = append(m.inputs, req.Input)
	if m.failure != nil {
		return backend.DecisionResult{}, m.failure
	}
	answers := make(map[string]backend.DecisionAnswer, len(req.Questions))
	for _, q := range req.Questions {
		answers[q.ID] = m.answer(q)
	}
	return backend.DecisionResult{ID: "dec", Model: "fake", Answers: answers}, nil
}

func (m *fakeModel) answer(q backend.DecisionQuestion) backend.DecisionAnswer {
	switch {
	case strings.HasPrefix(q.ID, "group_"):
		return backend.DecisionAnswer{Type: "predicate", Probability: likelihood(m.groupOf(q.Question), m.groups, 0.05)}
	case strings.HasPrefix(q.ID, "recall_"):
		probabilities := make(map[string]float64, len(q.Options))
		for _, option := range q.Options {
			if option == noneOfThese {
				probabilities[option] = 0.5
				continue
			}
			probabilities[option] = m.glyphs[glyphOf(option)] / 4
		}
		return backend.DecisionAnswer{Type: "choice", Choice: noneOfThese, Probabilities: probabilities}
	default:
		label := strings.TrimSuffix(strings.TrimPrefix(q.Question, "Is "), " a good match for the search text?")
		return backend.DecisionAnswer{Type: "predicate", Probability: likelihood(glyphOf(label), m.glyphs, 0)}
	}
}

// groupOf returns the group a group question names.
func (m *fakeModel) groupOf(question string) string {
	for group := range m.groups {
		if strings.Contains(question, `"`+group+`"`) {
			return group
		}
	}
	return ""
}

// likelihood returns the set probability of key, or fallback when the test set none.
func likelihood(key string, table map[string]float64, fallback float64) float64 {
	if p, ok := table[key]; ok {
		return p
	}
	return fallback
}

// glyphOf returns the glyph a label starts with.
func glyphOf(label string) string {
	glyph, _, _ := strings.Cut(label, " ")
	return glyph
}

// errUnavailable is the error of a decision route that cannot answer.
var errUnavailable = errors.New("decision route unavailable")

// testCatalog is a small catalog with the groups the tests need: two semantic groups, a lexical-only group and one
// group that the tests leave unlikely.
var testCatalog = []Record{
	{Key: "cat", Glyph: "🐱", Name: "Cat Face", Group: "Animals & Nature"},
	{Key: "tiger", Glyph: "🐯", Name: "Tiger Face", Group: "Animals & Nature"},
	{Key: "dog", Glyph: "🐶", Name: "Dog Face", Group: "Animals & Nature"},
	{Key: "pleading", Glyph: "🥺", Name: "Pleading Face", Group: "Smileys & Emotion"},
	{Key: "flag", Glyph: "🇫🇷", Name: "France", Group: "Flags"},
	{Key: "receipt", Glyph: "🧾", Name: "Receipt", Group: "Objects"},
}

// sayingCatalog is a catalog over fmt.Sprintf'd names, for chunking tests.
func sayingCatalog(count int) []Record {
	records := make([]Record, 0, count)
	for i := range count {
		records = append(records, Record{Key: fmt.Sprint(i), Glyph: fmt.Sprintf("g%d", i), Name: fmt.Sprintf("Animal %d", i), Group: "Animals & Nature"})
	}
	return records
}
