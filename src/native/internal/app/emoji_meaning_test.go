package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/charsearch"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// pleadingModel answers the decision the way a model does for "smile": every group is likely, and the pleading face is
// the only character that matches. It counts its calls, so a test can tell whether the model was asked.
type pleadingModel struct {
	calls   int
	failure error
	// match names the character the model finds; empty means the pleading face.
	match string
}

// target is the label the model's match is written with.
func (m *pleadingModel) target() string {
	if m.match == "" {
		return "Pleading Face"
	}
	return m.match
}

func (m *pleadingModel) Decisions(_ context.Context, req backend.DecisionRequest) (backend.DecisionResult, error) {
	m.calls++
	if m.failure != nil {
		return backend.DecisionResult{}, m.failure
	}
	answers := make(map[string]backend.DecisionAnswer, len(req.Questions))
	for _, q := range req.Questions {
		switch q.Type {
		case "predicate":
			answers[q.ID] = backend.DecisionAnswer{Type: "predicate", Probability: pleadingLikelihood(q.Question, m.target())}
		case "choice":
			answers[q.ID] = backend.DecisionAnswer{Type: "choice", Choice: "none of these", Probabilities: pleadingChoices(q.Options, m.target())}
		}
	}
	return backend.DecisionResult{ID: "dec", Model: "test", Answers: answers}, nil
}

// pleadingLikelihood is the yes probability of a predicate: groups and the pleading face are likely, the rest not.
func pleadingLikelihood(question, target string) float64 {
	if strings.Contains(question, target) || strings.Contains(question, "category") {
		return 0.9
	}
	return 0.1
}

// pleadingChoices ranks the pleading face first among the options of a choice question.
func pleadingChoices(options []string, target string) map[string]float64 {
	probabilities := make(map[string]float64, len(options))
	for _, option := range options {
		probabilities[option] = 0
		if strings.Contains(option, target) {
			probabilities[option] = 0.9
		}
	}
	return probabilities
}

// meaningPalette is the goldens' palette with the picker's meaning batch answered by decider, with no typing pause.
func meaningPalette(t *testing.T, decider charsearch.Decider) *App {
	t.Helper()
	p := goldenPalette(t)
	zero := time.Duration(0)
	p.characterSearch = &characterSearch{decide: func() charsearch.Decider { return decider }, pause: &zero}
	return p
}

// TestEmojiMeaningBatchFollowsTheCatalogMatches searches "crying", which the catalog answers with other faces, and
// expects the model's pleading face after them.
func TestEmojiMeaningBatchFollowsTheCatalogMatches(t *testing.T) {
	model := &pleadingModel{}
	p := meaningPalette(t, model)
	openEmojiPicker(p)
	goldenQuery(p, "crying")
	titles := rowTitles(p)
	if model.calls == 0 {
		t.Fatal("the model was never asked about the search text")
	}
	if len(titles) < 2 || titles[len(titles)-1] != "Pleading Face" || slices.Index(titles, "Pleading Face") != len(titles)-1 {
		t.Fatalf("rows = %v, want the meaning match last and once", titles)
	}
}

// TestEmojiMeaningMatchAlreadyInTheCatalogIsNotRepeated searches "smile", where the catalog already lists the pleading
// face, and expects the picker to show exactly what it shows without an account.
func TestEmojiMeaningMatchAlreadyInTheCatalogIsNotRepeated(t *testing.T) {
	signedOut := goldenPalette(t)
	openEmojiPicker(signedOut)
	goldenQuery(signedOut, "smile")

	p := meaningPalette(t, &pleadingModel{})
	openEmojiPicker(p)
	goldenQuery(p, "smile")
	if got, want := rowTitles(p), rowTitles(signedOut); !slices.Equal(got, want) {
		t.Fatalf("rows differ from the catalog's own matches: got %d rows, want %d", len(got), len(want))
	}
}

// TestSignedOutPickerHasNoMeaningBatch checks the picker without an account adds no rows for text the catalog does not
// answer, while the same text with an account gets the model's match.
func TestSignedOutPickerHasNoMeaningBatch(t *testing.T) {
	signedOut := goldenPalette(t)
	openEmojiPicker(signedOut)
	goldenQuery(signedOut, "zzqx")
	if titles := rowTitles(signedOut); len(titles) != 0 {
		t.Fatalf("rows = %v, want no rows without an account", titles)
	}

	p := meaningPalette(t, &pleadingModel{})
	openEmojiPicker(p)
	goldenQuery(p, "zzqx")
	if titles := rowTitles(p); !slices.Equal(titles, []string{"Pleading Face"}) {
		t.Fatalf("rows = %v, want the model's match with an account", titles)
	}
}

// TestFailingModelLeavesTheCatalogMatches checks a failed decision adds nothing and shows nothing.
func TestFailingModelLeavesTheCatalogMatches(t *testing.T) {
	signedOut := goldenPalette(t)
	openEmojiPicker(signedOut)
	goldenQuery(signedOut, "crying")
	want := rowTitles(signedOut)

	p := meaningPalette(t, &pleadingModel{failure: errors.New("rate limited")})
	openEmojiPicker(p)
	goldenQuery(p, "crying")
	if got := rowTitles(p); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want the catalog matches %v", got, want)
	}
}

// TestEmojiMeaningGrid is the picker with the meaning batch after the catalog's matches, as the golden shows it.
func TestEmojiMeaningGrid(t *testing.T) {
	p := meaningPalette(t, &pleadingModel{})
	openEmojiPicker(p)
	goldenQuery(p, "crying")
	golden.AssertMatrix(t, "emoji-meaning", p.View, paletteGoldens)
}
