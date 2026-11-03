package search

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// The cases below are ported from src/app/electron/search/action-ranking.test.ts (the ones that are about
// title scoring and boosts; the rest concern aliases, markers and the 1.x app priority, which the Go root
// does not have). The root-level cases live in internal/app/root_ranking_test.go.

// TestPortedAIBuilderContinuationTitleDoesNotExactMatch ports "AI builder chat continuation title no longer
// exact-matches the prompt".
func TestPortedAIBuilderContinuationTitleDoesNotExactMatch(t *testing.T) {
	const query = "quit all apps"
	if got := Score("Quit All Apps", query); got != 100 {
		t.Fatalf("Score(Quit All Apps) = %d, want 100", got)
	}
	if got := Score("Continue AI chat: quit all apps", query); got >= 100 {
		t.Fatalf("Score(Continue AI chat: quit all apps) = %d, want below 100", got)
	}
}

// TestPortedTitlePrefixesOutrankWholeWordsInLongerTitles ports "title prefixes outrank whole-word matches in
// longer titles" on the 1.x title scale.
func TestPortedTitlePrefixesOutrankWholeWordsInLongerTitles(t *testing.T) {
	if !(PrioritizedTitleScore("Dictate", "dictat") > PrioritizedTitleScore(`Automate "dictat"`, "dictat")) {
		t.Errorf("Dictate should outrank Automate \"dictat\" for dictat: %d vs %d",
			PrioritizedTitleScore("Dictate", "dictat"), PrioritizedTitleScore(`Automate "dictat"`, "dictat"))
	}
	if !(PrioritizedTitleScore("Dictate", "dictate") > PrioritizedTitleScore("Dictation Settings", "dictate")) {
		t.Errorf("Dictate should outrank Dictation Settings for dictate: %d vs %d",
			PrioritizedTitleScore("Dictate", "dictate"), PrioritizedTitleScore("Dictation Settings", "dictate"))
	}
}

// TestPrioritizedTitleScoreBands pins the 1.x band values that PrioritizedTitleScore maps the bands to.
func TestPrioritizedTitleScoreBands(t *testing.T) {
	tests := []struct {
		title, query string
		want         int
	}{
		{"hello", "hello", 1000},
		{"hello world", "hello", 950},
		{"rightwards arrow", "right arrow", 925},
		{"hello world", "world", 900},
		{"move rightwards arrow", "right arrow", 875},
		{"rightwards arrow", "arrow right", 860},
		{"party", "art", 850},
		{"hello", "hlo", 820},
		{"gmail", "gmial", 810},
		{"hello", "xyz", 0},
		{"hello", "", 0},
	}
	for _, tt := range tests {
		if got := PrioritizedTitleScore(tt.title, tt.query); got != tt.want {
			t.Errorf("PrioritizedTitleScore(%q, %q) = %d, want %d", tt.title, tt.query, got, tt.want)
		}
	}
}

// TestBoostNeverSkipsABand is the frecency fairness rule: an exact match beats any boosted partial match,
// and no boost lifts a row past the next band above it.
func TestBoostNeverSkipsABand(t *testing.T) {
	if got := Boosted(95, 10_000); got >= 100 {
		t.Fatalf("a boosted 95 reached the exact band: %d", got)
	}
	for _, score := range bands[:len(bands)-1] {
		for boost := 0; boost <= 1_000; boost += 7 {
			got := Boosted(score, boost)
			if above := bandAbove(score); got >= above {
				t.Fatalf("Boosted(%d, %d) = %d reached band %d", score, boost, got, above)
			}
			if got < score {
				t.Fatalf("Boosted(%d, %d) = %d lowered the score", score, boost, got)
			}
		}
	}
}

func TestBoostedAddsWholePointsWithinTheHeadroom(t *testing.T) {
	tests := []struct{ score, boost, want int }{
		{80, 0, 80},
		{80, 9, 80},  // less than a point
		{80, 50, 85}, // five points, below the 90 band
		{80, 1_000, 89},
		{95, 1_000, 99},
		{100, 1_000, 200}, // the top band has nothing above it
		{20, 1_000, 49},
		{10, 1_000, 19},
		{50, -40, 50}, // a negative boost is no boost
	}
	for _, tt := range tests {
		if got := Boosted(tt.score, tt.boost); got != tt.want {
			t.Errorf("Boosted(%d, %d) = %d, want %d", tt.score, tt.boost, got, tt.want)
		}
	}
}

// fixedLearner boosts the keys it knows by a fixed amount and records what it is told.
type fixedLearner struct {
	boosts   map[string]int
	recorded []string
}

func (l *fixedLearner) Boost(key, _ string) int { return l.boosts[key] }

func (l *fixedLearner) Record(key, query string) { l.recorded = append(l.recorded, key+"|"+query) }

// TestCoordinatorRaisesOnlyLearnedRows checks the coordinator's share of learning: a keyed row is boosted
// once per batch, and an unkeyed row keeps its score.
func TestCoordinatorRaisesOnlyLearnedRows(t *testing.T) {
	learner := &fixedLearner{boosts: map[string]int{"app:b": 1_000}}
	rows := []Item{
		{Title: "alpha", Score: 80, LearnKey: "app:a"},
		{Title: "beta", Score: 80, LearnKey: "app:b"},
		{Title: "gamma", Score: 80},
	}
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true, Learner: learner})
	snap, ok := c.Query("x", []Provider{NewRowsProvider("p", func(context.Context, string) []Item { return rows })})
	if !ok {
		t.Fatal("a synchronous query returns its rows")
	}
	got := map[string]int{}
	for _, it := range snap.Items {
		got[it.Title] = it.Score
	}
	if got["beta"] != 89 || got["alpha"] != 80 || got["gamma"] != 80 {
		t.Fatalf("scores = %v, want beta 89 and the others 80", got)
	}
	if snap.Items[0].Title != "beta" {
		t.Fatalf("the boosted row should sort first, got %q", snap.Items[0].Title)
	}
}
