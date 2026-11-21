package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.learnings is the dev fixture of the AI Learnings list: the populated list with its Edit and confirmed Delete
// actions, the editor, and the empty state. It renders the same shapes as the nevermind.ai-learnings built-in from static
// rules, because only built-in extensions may call ctx.Learnings. Saving shows the stale refusal the host gives when a
// review changed the rule meanwhile.

var rules = []api.LearningRule{
	{ID: "r1", Kind: "preference", Text: "Prefer short command titles", Revision: 1},
	{ID: "r2", Kind: "workflow", Text: "Show a list before an editor when several items can be chosen", AppliesWhen: "building list views", Revision: 1},
	{ID: "r3", Kind: "environment", Text: "Pablo's keyboard is Spanish, so avoid punctuation shortcuts", Revision: 3},
}

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{
		{ID: "list", Title: "Learnings fixture"},
		{ID: "empty", Title: "Learnings fixture: empty"},
	}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "list":
		return ctx.Navigation().Push(listView(rules)), nil
	case "empty":
		return ctx.Navigation().Push(listView(nil)), nil
	case "edit":
		view := api.EditorView("Edit Learning", in.Ref.Args["text"], api.Run("save", "Save", in.Ref.Args))
		view.Editor.Placeholder = "Describe the rule"
		return ctx.Navigation().Push(&view), nil
	case "save":
		message := "The learnings changed. Nothing was saved; the list shows the current rules."
		return api.Result{Navigation: api.NavPop, Toast: &api.Toast{Message: message, Tone: "error"}}, nil
	case "delete":
		return api.Result{Toast: &api.Toast{Message: "Learning deleted", Tone: "success"}}, nil
	}
	return api.Result{}, fmt.Errorf("unknown command %q", in.Ref.ID)
}

func listView(all []api.LearningRule) *api.View {
	items := make([]api.Item, 0, len(all))
	for _, rule := range all {
		args := map[string]string{"id": rule.ID, "text": rule.Text}
		edit := api.Run("edit", "Edit", args)
		remove := api.Destructive(api.WithConfirmation(api.Run("delete", "Delete", args), api.Confirmation{
			Title: "Delete this learning?", Message: "The AI builder will not learn it again.", ConfirmLabel: "Delete", Destructive: true,
		}))
		items = append(items, api.Item{ID: rule.ID, Title: rule.Text, Subtitle: rule.Kind, Icon: "brain", Primary: &edit, Actions: []api.Action{remove}})
	}
	view := api.ListView("AI Learnings", items...)
	view.EmptyView = &api.EmptyState{Title: "No learnings yet.", Subtitle: "The AI builder adds rules here as it learns your preferences."}
	return &view
}
