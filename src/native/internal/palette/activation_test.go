package palette

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// activationRoot is a root with one learnable row whose second action is the one the test drives.
func activationRoot(second search.Action) *View {
	return &View{Title: "Nevermind", Search: func(string) []search.Item {
		return []search.Item{{Title: "Beta", LearnKey: "app:beta", Actions: []search.Action{
			{Title: "Open", Run: func(search.Host) {}},
			second,
		}}}
	}}
}

func TestEveryWayToActivateARowLearnsItExactlyOnce(t *testing.T) {
	t.Run("action menu", func(t *testing.T) {
		learner := &recordingLearner{}
		h := newHarness(t, activationRoot(search.Action{Title: "Second", Run: func(search.Host) {}}), true, func(o *Options) { o.Learner = learner })
		h.typeText("bet")
		h.key(Mod, ui.KeyK)
		h.key(0, ui.KeyDown)
		h.key(0, ui.KeyEnter)
		if !slices.Equal(learner.calls, []string{"app:beta|bet"}) {
			t.Fatalf("learner calls = %v", learner.calls)
		}
	})
	t.Run("action shortcut", func(t *testing.T) {
		learner := &recordingLearner{}
		second := search.Action{Title: "Second", Mods: Mod, Key: ui.KeyJ, Run: func(search.Host) {}}
		h := newHarness(t, activationRoot(second), true, func(o *Options) { o.Learner = learner })
		h.typeText("bet")
		h.key(Mod, ui.KeyJ)
		if !slices.Equal(learner.calls, []string{"app:beta|bet"}) {
			t.Fatalf("learner calls = %v", learner.calls)
		}
	})
	t.Run("page action is not the row's", func(t *testing.T) {
		learner := &recordingLearner{}
		root := activationRoot(search.Action{Title: "Second", Run: func(search.Host) {}})
		root.Actions = func() []search.Action {
			return []search.Action{{Title: "Page", Mods: Mod, Key: ui.KeyJ, Run: func(search.Host) {}}}
		}
		h := newHarness(t, root, true, func(o *Options) { o.Learner = learner })
		h.typeText("bet")
		h.key(Mod, ui.KeyJ)
		if len(learner.calls) != 0 {
			t.Fatalf("a page action learned the chosen row: %v", learner.calls)
		}
	})
}

// outcomeRoot is a root whose row's Enter action ends later: it hands its completion to the test.
func outcomeRoot(finish *func(bool) bool) *View {
	return &View{Title: "Nevermind", Search: func(string) []search.Item {
		return []search.Item{{Title: "Beta", LearnKey: "app:beta", Actions: []search.Action{
			{Title: "Open", Run: func(h search.Host) { *finish = h.(search.ActionOutcomeHost).BeginActionOutcome() }},
		}}}
	}}
}

func TestAnAsynchronousActionLearnsOnlyWhenItSucceedsWhileCurrent(t *testing.T) {
	cases := []struct {
		name   string
		end    func(h *harness, finish func(bool) bool)
		learns bool
	}{
		{"succeeds", func(h *harness, finish func(bool) bool) { finish(true) }, true},
		{"succeeds twice", func(h *harness, finish func(bool) bool) { finish(true); finish(true) }, true},
		{"fails", func(h *harness, finish func(bool) bool) { finish(false) }, false},
		{"is cancelled by navigation", func(h *harness, finish func(bool) bool) { h.p.cancelAction(); finish(true) }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			learner := &recordingLearner{}
			var finish func(bool) bool
			root := outcomeRoot(&finish)
			h := newHarness(t, root, true, func(o *Options) { o.Learner = learner })
			h.typeText("bet")
			h.key(0, ui.KeyEnter)
			if finish == nil {
				t.Fatal("the action never began")
			}
			if len(learner.calls) != 0 {
				t.Fatalf("learned %v before the action ended", learner.calls)
			}
			tc.end(h, finish)
			want := []string(nil)
			if tc.learns {
				want = []string{"app:beta|bet"}
			}
			if !slices.Equal(learner.calls, want) {
				t.Fatalf("learner calls = %v, want %v", learner.calls, want)
			}
		})
	}
}

func TestAFailedOrSupersededSynchronousRunStillLearnsOnlyOnce(t *testing.T) {
	learner := &recordingLearner{}
	var probe func() bool
	var finish func(bool) bool
	root := &View{Title: "Nevermind", Search: func(string) []search.Item {
		return []search.Item{{Title: "Beta", LearnKey: "app:beta", Actions: []search.Action{{Title: "Open", Run: func(h search.Host) {
			outcome := h.(search.ActionOutcomeHost)
			finish = outcome.BeginActionOutcome()
			probe = outcome.ActionCurrent()
		}}}}}
	}}
	h := newHarness(t, root, true, func(o *Options) { o.Learner = learner })
	h.typeText("bet")
	h.key(0, ui.KeyEnter)
	if !probe() {
		t.Fatal("the action just begun is not current")
	}
	h.p.cancelAction()
	if probe() || finish(true) || len(learner.calls) != 0 {
		t.Fatalf("a cancelled action stayed current or learned: %v", learner.calls)
	}
}
