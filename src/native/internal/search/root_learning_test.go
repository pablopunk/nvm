package search

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestEmptyRootUsesFrecencyWithoutTypedMatchBandCaps(t *testing.T) {
	learner := &fixedLearner{boosts: map[string]int{}}
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true, Learner: learner, Flat: true})
	provider := NewRowsProvider("fixture", func(context.Context, string) []Item {
		return []Item{
			{Title: "App", Score: 55, LearnKey: "app:test"},
			{Title: "Extension", Score: 12, LearnKey: "ext:test"},
		}
	})
	assertFirst := func(want string) {
		t.Helper()
		snap, _ := c.Query("", []Provider{provider})
		if snap.Items[0].Title != want {
			t.Fatalf("root first = %q, want %q", snap.Items[0].Title, want)
		}
	}
	assertFirst("App")
	learner.boosts["ext:test"] = 60
	assertFirst("Extension")
	learner.boosts["app:test"] = 32
	assertFirst("App")
}
