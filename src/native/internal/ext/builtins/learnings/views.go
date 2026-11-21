package learnings

import (
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

const (
	searchPlaceholder = "Search Learnings"
	emptyTitle        = "No learnings yet."
	emptySubtitle     = "The AI builder adds rules here as it learns your preferences."
	titleRunes        = 90
)

// listView is the list of rules, with the empty state only here.
func listView(list api.LearningList) api.View {
	view := api.ListView(listTitle, listItems(list)...)
	view.Placeholder = searchPlaceholder
	view.EmptyView = &api.EmptyState{Title: emptyTitle, Subtitle: emptySubtitle}
	view.Refresh = &api.Refresh{Ref: &api.ActionRef{ID: refRefresh}}
	return view
}

// editorView edits the text of one rule. Saving carries the revisions the list was read at.
func editorView(rule api.LearningRule, listRevision uint64) api.View {
	view := api.EditorView(editTitle, rule.Text, api.Run(refSave, "Save", ruleArgs(rule, listRevision)))
	view.Editor.Placeholder = "Describe the rule"
	return view
}

// listItems are the rows of the rules, in the order the store keeps them.
func listItems(list api.LearningList) []api.Item {
	items := make([]api.Item, 0, len(list.Rules))
	for _, rule := range list.Rules {
		items = append(items, ruleRow(rule, list.Revision))
	}
	return items
}

// ruleRow shows the rule; its primary action edits it and Delete asks first.
func ruleRow(rule api.LearningRule, listRevision uint64) api.Item {
	args := ruleArgs(rule, listRevision)
	edit := api.Run(refEdit, "Edit", args)
	remove := api.Destructive(api.WithConfirmation(api.Run(refDelete, "Delete", args), api.Confirmation{
		Title: "Delete this learning?", Message: "The AI builder will not learn it again.", ConfirmLabel: "Delete", Destructive: true,
	}))
	return api.Item{
		ID: rule.ID, Title: oneLine(rule.Text), Subtitle: subtitle(rule), Icon: "brain",
		Primary: &edit, Actions: []api.Action{remove}, Detail: &api.Detail{Text: rule.Text},
	}
}

func ruleArgs(rule api.LearningRule, listRevision uint64) map[string]string {
	return map[string]string{
		argID: rule.ID, argList: strconv.FormatUint(listRevision, 10), argRule: strconv.FormatUint(rule.Revision, 10),
	}
}

func subtitle(rule api.LearningRule) string {
	if rule.AppliesWhen == "" {
		return rule.Kind
	}
	return rule.Kind + "  ·  when " + oneLine(rule.AppliesWhen)
}

func oneLine(text string) string {
	line := strings.Join(strings.Fields(text), " ")
	if runes := []rune(line); len(runes) > titleRunes {
		return string(runes[:titleRunes]) + "…"
	}
	return line
}
