package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// A concealed view holds a secret in the query. Enter runs its row and the query is neither remembered by the root
// history nor recorded by the learner.
func TestConcealedViewDoesNotRememberOrLearnItsQuery(t *testing.T) {
	history := NewQueryHistory([]string{"alpha"}, nil)
	learner := &recordingLearner{}
	root := &View{Title: "Nevermind", Search: func(string) []search.Item { return nil }}
	secret := &View{Title: "Token", Concealed: true, Search: func(q string) []search.Item {
		return []search.Item{{Title: "Set Token", LearnKey: "form:token", Actions: []search.Action{
			{Title: "Set", Run: func(search.Host) {}},
		}}}
	}}
	h := newHarness(t, root, true, func(o *Options) { o.History = history; o.Learner = learner })
	h.p.Push(secret)
	h.typeText("hunter2")
	h.key(0, ui.KeyEnter)

	if got := history.Entries(); len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("history = %v; want the secret left out", got)
	}
	if len(learner.calls) != 0 {
		t.Fatalf("learner calls = %v; want none for a concealed query", learner.calls)
	}
}
