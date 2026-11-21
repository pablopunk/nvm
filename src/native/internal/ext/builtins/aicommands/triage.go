package aicommands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TriageCommandID is the command that asks three questions of the selection. Its global id is
// "extension:<ID>:<TriageCommandID>", the key 1.x stored its shortcut under.
const TriageCommandID = "triage-selected-text-with-ai"

// TriageTitle is the title the palette and the indicator show for the triage.
const TriageTitle = "Triage Selected Text with AI"

// triageSubtitle is the row's subtitle, and triageDismissMs how long a result stays: long enough to read three answers.
const (
	triageSubtitle  = "Reply needed, category and urgency in one call"
	triageDismissMs = 6000
)

// triageQuestions are the three questions a triage asks of the selected text, answered in one decision call.
var triageQuestions = []api.DecisionQuestion{
	{ID: "needs_reply", Type: api.DecisionPredicate, Question: "Does the text ask the reader for a reply or an action?"},
	{ID: "category", Type: api.DecisionChoice, Question: "Which category best fits the text?", Options: []string{"bug report", "feature request", "question", "other"}},
	{ID: "urgency", Type: api.DecisionScore, Question: "How urgent is the text?", Min: intPointer(1), Max: intPointer(5), Labels: map[string]string{"1": "no rush", "5": "drop everything"}},
}

// triage reads the selection, asks the decision route about it, and shows the answers in the indicator. It needs an
// account before it reads anything, so a signed-out triage explains itself without touching the selection.
func (e Extension) triage(ctx api.Context) error {
	hud := newHUD(ctx, TriageTitle)
	ai := ctx.AI()
	available, err := ai.IsAvailable()
	if err != nil {
		return hud.fail(err)
	}
	if !available {
		return hud.fail(errNotSignedIn)
	}
	hud.show("Reading Selection")
	text, found, err := ctx.Desktop().Selection().Text()
	if err != nil {
		return hud.fail(err)
	}
	if !found || strings.TrimSpace(text) == "" {
		return hud.fail(errNoSelection)
	}
	hud.show("Deciding")
	input, err := json.Marshal(text)
	if err != nil {
		return hud.fail(err)
	}
	result, err := ai.Decide(api.DecisionRequest{Input: input, Questions: triageQuestions})
	if err != nil {
		return hud.fail(err)
	}
	return hud.succeed(triageSummary(result.Answers), triageDismissMs)
}

// triageSummary is the one line the indicator shows for a triage: the category, the urgency and whether a reply is needed,
// joined with middle dots. Answers that are missing or of another type are left out.
func triageSummary(answers map[string]api.DecisionAnswer) string {
	var parts []string
	if category, ok := answers["category"]; ok && category.Type == api.DecisionChoice {
		parts = append(parts, category.Choice)
	}
	if urgency, ok := answers["urgency"]; ok && urgency.Type == api.DecisionScore {
		parts = append(parts, fmt.Sprintf("urgency %d/5", int(urgency.Score)))
	}
	if needsReply, ok := answers["needs_reply"]; ok && needsReply.Type == api.DecisionPredicate {
		if needsReply.Flag {
			parts = append(parts, "needs reply")
		} else {
			parts = append(parts, "no reply needed")
		}
	}
	if len(parts) == 0 {
		return "No triage result"
	}
	return strings.Join(parts, " · ")
}

func intPointer(value int) *int {
	return &value
}

// triageCommand is the row of the triage: a background run that dismisses the palette, as the fix does.
func triageCommand() api.Command {
	return api.Command{
		ID: TriageCommandID, Title: TriageTitle, Subtitle: triageSubtitle, Icon: "sparkles",
		Aliases:         []string{"triage text", "classify selected text", "decide"},
		Keywords:        []string{"triage classify decide reply urgency selected text ai"},
		Mode:            api.ModeBackground,
		DismissAfterRun: true,

		Placement: []api.Placement{api.PlacementRoot, api.PlacementSearch},
	}
}
