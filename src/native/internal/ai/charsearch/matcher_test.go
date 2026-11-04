package charsearch

import (
	"context"
	"testing"
)

// cuteModel is the model of the picker's "cute" search: animals and smileys are likely, and the faces are cute to
// different degrees.
func cuteModel() *fakeModel {
	return &fakeModel{
		groups: map[string]float64{"Animals & Nature": 0.9, "Smileys & Emotion": 0.9, "Objects": 0.05},
		glyphs: map[string]float64{"🐱": 0.95, "🥺": 0.7, "🐯": 0.4, "🐶": 0.6},
	}
}

func recordKeys(records []Record) []string {
	keys := make([]string, 0, len(records))
	for _, r := range records {
		keys = append(keys, r.Key)
	}
	return keys
}

func sameKeys(got []Record, want ...string) bool {
	keys := recordKeys(got)
	if len(keys) != len(want) {
		return false
	}
	for i := range keys {
		if keys[i] != want[i] {
			return false
		}
	}
	return true
}

func TestMatchesNarrowsByCategoryShortlistsByChoiceAndKeepsVerifiedMatchesBestFirst(t *testing.T) {
	model := cuteModel()
	matches, err := NewMatcher(testCatalog).Matches(context.Background(), model, allCategory, "Cute ")
	if err != nil {
		t.Fatal(err)
	}
	if !sameKeys(matches, "cat", "pleading", "dog") {
		t.Fatalf("matches = %v, want cat, pleading, dog: best first, tiger below the threshold", recordKeys(matches))
	}
	if len(model.calls) != 3 {
		t.Fatalf("decision calls = %d, want the three steps", len(model.calls))
	}
	if len(model.calls[1]) > maxQuestions {
		t.Fatalf("shortlist asked %d questions, want at most %d", len(model.calls[1]), maxQuestions)
	}
	for _, q := range model.calls[1] {
		if len(q.Options) > choicesPerQuestion+1 {
			t.Fatalf("choice question offers %d options, want at most 32", len(q.Options))
		}
	}
}

func TestTiedCharactersKeepTheOrderTheyWereOffered(t *testing.T) {
	model := &fakeModel{
		groups: map[string]float64{"Animals & Nature": 0.9},
		glyphs: map[string]float64{"🐱": 0.8, "🐯": 0.8, "🐶": 0.8},
	}
	matches, err := NewMatcher(testCatalog).Matches(context.Background(), model, allCategory, "pet animals")
	if err != nil {
		t.Fatal(err)
	}
	if !sameKeys(matches, "cat", "tiger", "dog") {
		t.Fatalf("matches = %v, want the offered order for equal probabilities", recordKeys(matches))
	}
}

func TestAChosenCategorySkipsTheCategoryStep(t *testing.T) {
	model := &fakeModel{glyphs: map[string]float64{"🐱": 0.95}}
	matches, err := NewMatcher(testCatalog).Matches(context.Background(), model, "Animals & Nature", "adorable pet")
	if err != nil {
		t.Fatal(err)
	}
	if !sameKeys(matches, "cat") {
		t.Fatalf("matches = %v, want the cat", recordKeys(matches))
	}
	if len(model.calls) != 2 || model.calls[0][0].ID != "recall_0" {
		t.Fatalf("calls = %d, want the recall step first and then the verification", len(model.calls))
	}
}

func TestLexicalOnlyCategoriesNeverAskTheModel(t *testing.T) {
	model := &fakeModel{}
	matches, err := NewMatcher(testCatalog).Matches(context.Background(), model, "Flags", "france")
	if err != nil || len(matches) != 0 || len(model.calls) != 0 {
		t.Fatalf("matches = %v, err = %v, calls = %d; want nothing and no call", recordKeys(matches), err, len(model.calls))
	}
}

func TestShortTextNeverAsksTheModel(t *testing.T) {
	model := &fakeModel{}
	matches, err := NewMatcher(testCatalog).Matches(context.Background(), model, allCategory, "ox")
	if err != nil || len(matches) != 0 || len(model.calls) != 0 {
		t.Fatalf("matches = %v, err = %v, calls = %d; want nothing and no call", recordKeys(matches), err, len(model.calls))
	}
}

func TestARepeatedSearchIsAnsweredFromMemory(t *testing.T) {
	model := cuteModel()
	matcher := NewMatcher(testCatalog)
	if _, err := matcher.Matches(context.Background(), model, allCategory, "kitten time"); err != nil {
		t.Fatal(err)
	}
	calls := len(model.calls)
	if _, err := matcher.Matches(context.Background(), model, allCategory, "Kitten Time"); err != nil {
		t.Fatal(err)
	}
	if len(model.calls) != calls {
		t.Fatalf("calls grew from %d to %d for a repeated search", calls, len(model.calls))
	}
}

func TestTheSearchStopsAfterTheCategoryStepWhenNoCategoryIsLikely(t *testing.T) {
	model := &fakeModel{groups: map[string]float64{"Animals & Nature": 0.01, "Smileys & Emotion": 0.01}}
	matches, err := NewMatcher(testCatalog).Matches(context.Background(), model, allCategory, "quarterly invoices")
	if err != nil || len(matches) != 0 {
		t.Fatalf("matches = %v, err = %v; want nothing", recordKeys(matches), err)
	}
	if len(model.calls) != 1 {
		t.Fatalf("calls = %d, want only the category step", len(model.calls))
	}
}

func TestAFailedDecisionReturnsItsErrorAndRemembersNothing(t *testing.T) {
	model := &fakeModel{failure: errUnavailable}
	matcher := NewMatcher(testCatalog)
	if _, err := matcher.Matches(context.Background(), model, allCategory, "kitten time"); err == nil {
		t.Fatal("err = nil, want the decision failure")
	}
	model.failure = nil
	model.groups = map[string]float64{"Animals & Nature": 0.9}
	if _, err := matcher.Matches(context.Background(), model, allCategory, "kitten time"); err != nil {
		t.Fatalf("second search: %v", err)
	}
	if len(model.calls) < 2 {
		t.Fatal("a failed search was remembered, so the retry asked nothing")
	}
}

func TestTheInputQuotesTheSearchTextAsJSON(t *testing.T) {
	model := &fakeModel{}
	if _, err := NewMatcher(testCatalog).Matches(context.Background(), model, allCategory, `Say "Hi"`); err != nil {
		t.Fatal(err)
	}
	if len(model.inputs) == 0 || model.inputs[0] != `Search text: "say \"hi\""` {
		t.Fatalf("input = %v, want the lower-cased text quoted as JSON", model.inputs)
	}
}

func TestTheShortlistAsksInChunksOfThirtyOneCharacters(t *testing.T) {
	model := &fakeModel{groups: map[string]float64{"Animals & Nature": 0.9}}
	if _, err := NewMatcher(sayingCatalog(40)).Matches(context.Background(), model, allCategory, "animal"); err != nil {
		t.Fatal(err)
	}
	if len(model.calls) != 2 {
		t.Fatalf("calls = %d, want the group step and the recall step only", len(model.calls))
	}
	if got := len(model.calls[1]); got != 2 {
		t.Fatalf("recall questions = %d, want 40 characters in two chunks", got)
	}
	if got := len(model.calls[1][0].Options); got != choicesPerQuestion+1 {
		t.Fatalf("first chunk options = %d, want 31 characters and none of these", got)
	}
}

var _ Decider = (*fakeModel)(nil)
