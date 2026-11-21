package app

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/charsearch"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// pleadingModel answers the decision the way a model does for "crying": every group is likely, and the pleading face is
// the only character that matches. It counts its calls, so a test can tell whether the model was asked.
type pleadingModel struct {
	calls int
}

func (m *pleadingModel) Decisions(_ context.Context, req backend.DecisionRequest) (backend.DecisionResult, error) {
	m.calls++
	answers := make(map[string]backend.DecisionAnswer, len(req.Questions))
	for _, q := range req.Questions {
		switch q.Type {
		case "predicate":
			answers[q.ID] = backend.DecisionAnswer{Type: "predicate", Probability: pleadingLikelihood(q.Question)}
		case "choice":
			answers[q.ID] = backend.DecisionAnswer{Type: "choice", Choice: "none of these", Probabilities: pleadingChoices(q.Options)}
		}
	}
	return backend.DecisionResult{ID: "dec", Model: "test", Answers: answers}, nil
}

// pleadingLikelihood is the yes probability of a predicate: groups and the pleading face are likely, the rest not.
func pleadingLikelihood(question string) float64 {
	if strings.Contains(question, "Pleading Face") || strings.Contains(question, "category") {
		return 0.9
	}
	return 0.1
}

// pleadingChoices ranks the pleading face first among the options of a choice question.
func pleadingChoices(options []string) map[string]float64 {
	probabilities := make(map[string]float64, len(options))
	for _, option := range options {
		probabilities[option] = 0
		if strings.Contains(option, "Pleading Face") {
			probabilities[option] = 0.9
		}
	}
	return probabilities
}

// silentModel never answers, as the model does while it thinks: the meaning batch stays in flight.
type silentModel struct {
	done chan struct{}
}

func (m silentModel) Decisions(context.Context, backend.DecisionRequest) (backend.DecisionResult, error) {
	<-m.done
	return backend.DecisionResult{}, context.Canceled
}

// meaningPicker opens the Emoji & Symbols picker over the goldens' palette, with decider answering the meaning batch
// and the palette's clock under the test's control.
func meaningPicker(t *testing.T, decider charsearch.Decider) (*App, *queuedWindow, *fake.Clock) {
	t.Helper()
	p := goldenPalette(t)
	clock := p.ports.Clock.(*fake.Clock)
	set := fake.NewSet()
	p.exts.mgr.Store(newManagerOverNetwork(t, builtInExtensions(builtInInputs{Rates: p.rateCache, Zone: time.UTC, Clock: clock, EmojiDecider: decider}), set.Clock, set.PublicHTTP))
	win := openEmojiBuiltIn(t, p)
	return p, win, clock
}

// awaitMeaningRow advances the clock past the typing pause and drains the window until the row title shows.
func awaitMeaningRow(t *testing.T, p *App, win *queuedWindow, clock *fake.Clock, title string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		win.drain()
		if slices.Contains(rowTitles(p), title) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the meaning batch never showed %q, rows %v", title, rowTitles(p))
		}
		clock.Advance(time.Second)
		time.Sleep(time.Millisecond)
	}
}

// TestEmojiMeaningBatchFollowsTheCatalogMatches searches "crying", which the catalog answers with other faces, and
// expects the model's match last and once.
func TestEmojiMeaningBatchFollowsTheCatalogMatches(t *testing.T) {
	model := &pleadingModel{}
	p, win, clock := meaningPicker(t, model)
	goldenQuery(p, "crying")
	awaitMeaningRow(t, p, win, clock, "Pleading Face")
	titles := rowTitles(p)
	if model.calls == 0 {
		t.Fatal("the model was never asked about the search text")
	}
	if titles[len(titles)-1] != "Pleading Face" || slices.Index(titles, "Pleading Face") != len(titles)-1 {
		t.Fatalf("rows = %v, want the meaning match last and once", titles)
	}
}

// TestEmojiMeaningGrid is the picker with the meaning batch after the catalog's matches, as the golden shows it.
func TestEmojiMeaningGrid(t *testing.T) {
	p, win, clock := meaningPicker(t, &pleadingModel{})
	goldenQuery(p, "crying")
	awaitMeaningRow(t, p, win, clock, "Pleading Face")
	golden.AssertMatrix(t, "emoji-meaning", p.View, paletteGoldens)
}

// TestEmojiPickerShowsItsMeaningBatchLoadingAfterTheCatalogMatches is the grid while the meaning batch is in flight:
// the catalog's matches, then the batch as loading.
func TestEmojiPickerShowsItsMeaningBatchLoadingAfterTheCatalogMatches(t *testing.T) {
	model := silentModel{done: make(chan struct{})}
	t.Cleanup(func() { close(model.done) })
	p, win, clock := meaningPicker(t, model)
	goldenQuery(p, "crying")
	for range 10 {
		win.drain()
		clock.Advance(time.Second)
		time.Sleep(time.Millisecond)
	}
	win.drain()
	golden.AssertMatrix(t, "emoji-loading", p.View, paletteGoldens)
}

// TestSignedOutPickerHasNoMeaningBatch searches "smile" without a model and expects the catalog's own rows only.
func TestSignedOutPickerHasNoMeaningBatch(t *testing.T) {
	p, win, clock := meaningPicker(t, nil)
	goldenQuery(p, "smile")
	for range 10 {
		win.drain()
		clock.Advance(time.Second)
		time.Sleep(time.Millisecond)
	}
	win.drain()
	for _, row := range p.Results() {
		if row.Glyph == "" {
			t.Fatalf("row %q has no glyph; the picker showed a non-character row", row.Title)
		}
	}
}
