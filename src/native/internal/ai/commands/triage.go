package commands

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// TriageQuestions are the three questions a triage asks of the selected text, in one decision call.
var TriageQuestions = []backend.DecisionQuestion{
	{ID: "needs_reply", Type: "predicate", Question: "Does the text ask the reader for a reply or an action?"},
	{ID: "category", Type: "choice", Question: "Which category best fits the text?", Options: []string{"bug report", "feature request", "question", "other"}},
	{ID: "urgency", Type: "score", Question: "How urgent is the text?", Min: intPointer(1), Max: intPointer(5), Labels: map[string]string{"1": "no rush", "5": "drop everything"}},
}

// TriageSummary is the one line the indicator shows for a triage: category, urgency and whether a reply is needed,
// joined with middle dots. Answers that are missing are left out.
func TriageSummary(answers map[string]backend.DecisionAnswer) string {
	var parts []string
	if category, ok := answers["category"]; ok && category.Type == "choice" {
		parts = append(parts, category.Choice)
	}
	if urgency, ok := answers["urgency"]; ok && urgency.Type == "score" {
		parts = append(parts, fmt.Sprintf("urgency %d/5", int(urgency.Score)))
	}
	if needsReply, ok := answers["needs_reply"]; ok && needsReply.Type == "predicate" {
		if needsReply.Predicate {
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
