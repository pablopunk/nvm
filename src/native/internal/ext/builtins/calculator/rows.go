package calculator

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/search"
)

// score ranks the answer above every other row: a calculation is what the person typed.
const score = 1000

// pasteShortcut pastes the result, as the 1.x calculator did.
const pasteShortcut = "CommandOrControl+Enter"

// resultItem is the row of one answer: its copy actions, the continue and swap actions that put the answer back in the
// search field, and its detail facts.
func resultItem(r calc.Result) api.Item {
	paste := api.Action{Kind: api.ActionPasteText, Title: "Paste Result", Text: r.Copy, Shortcut: pasteShortcut}
	actions := []api.Action{api.SetQuery("Continue Calculation", r.Raw)}
	if r.SwapQuery != "" {
		actions = append(actions, api.SetQuery("Swap Units", r.SwapQuery))
	}
	if r.Raw != r.Display {
		actions = append(actions, api.CopyText("Copy Unformatted Result", r.Raw, false))
	}
	actions = append(actions, paste)
	return api.Item{
		ID: "calculation", Title: r.Title, Subtitle: r.Subtitle, Icon: "calculator",
		Section: search.KindCalculator, Score: score, AnswersQuery: true,
		Appearance: &api.Appearance{Foreground: api.ColorOrange},
		Detail:     &api.Detail{Facts: factsOf(r.Detail)},
		Primary:    ptr(api.CopyText("Copy Result", r.Copy, false)),
		Actions:    actions,
	}
}

// factsOf turns the detail lines of an answer into the facts a detail pane draws.
func factsOf(lines []calc.DetailLine) []api.Fact {
	facts := make([]api.Fact, 0, len(lines))
	for _, line := range lines {
		facts = append(facts, api.Fact{Label: line.Label, Value: line.Value})
	}
	return facts
}

func ptr[T any](v T) *T { return &v }

// answerChanged reports whether the answer now differs from the one on screen. shown is nil when there was none.
func answerChanged(now calc.Result, ok bool, shown *calc.Result) bool {
	switch {
	case shown == nil:
		return ok
	case !ok:
		return false
	}
	return now.Display != shown.Display || now.Subtitle != shown.Subtitle
}
