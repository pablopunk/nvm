package calculator

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/search"
)

// score ranks the answer above every other row: a calculation is what the person typed.
const score = 1000

// continueShortcut puts the raw answer back in the search field, and swapShortcut swaps the units, as the 1.x
// calculator bound them. Paste Result has no key, as in 1.x.
const (
	continueShortcut = "CommandOrControl+Enter"
	swapShortcut     = "CommandOrControl+Shift+Enter"
)

// resultItem is the row of one answer: its copy actions, the continue and swap actions that put the answer back in the
// search field, and its detail facts.
func resultItem(r calc.Result) api.Item {
	actions := []api.Action{withShortcut(api.SetQuery("Continue Calculation", r.Raw), continueShortcut)}
	if r.SwapQuery != "" {
		actions = append(actions, withShortcut(api.SetQuery("Swap Units", r.SwapQuery), swapShortcut))
	}
	if r.Raw != r.Display {
		actions = append(actions, api.CopyText("Copy Unformatted Result", r.Raw, false))
	}
	actions = append(actions, api.PasteText("Paste Result", r.Copy))
	return api.Item{
		Title: r.Title, Subtitle: r.Subtitle, Icon: "calculator",
		Section: search.KindCalculator, Score: score, AnswersQuery: true,
		Appearance: &api.Appearance{Foreground: api.ColorOrange},
		Detail:     &api.Detail{Facts: factsOf(r.Detail)},
		Primary:    ptr(api.CopyText("Copy Result", r.Copy, false)),
		Actions:    actions,
	}
}

// withShortcut gives a declarative action the accelerator the palette binds and labels it with.
func withShortcut(action api.Action, accelerator string) api.Action {
	action.Shortcut = accelerator
	return action
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
