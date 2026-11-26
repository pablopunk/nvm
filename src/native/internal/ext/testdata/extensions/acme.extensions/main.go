package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.extensions is the dev fixture of ctx.Extensions().OpenReview, the one call of the extensions service that shows
// host UI: the capability review of a disabled extension. The list shows the installed extensions; Enter on a disabled
// one opens its review, where only the user can enable it. With no host the list is empty and shows its empty state, and
// a refused review is an error toast.

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "list", Title: "Extensions fixture"}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "list":
		return ctx.Navigation().Push(listView(ctx.Extensions().List())), nil
	case "review":
		if err := ctx.Extensions().OpenReview(in.Ref.Args["id"]); err != nil {
			return api.Result{Toast: &api.Toast{Message: err.Error(), Tone: "error"}}, nil
		}
		return api.Result{}, nil
	}
	return api.Result{}, fmt.Errorf("unknown command %q", in.Ref.ID)
}

func listView(records []api.ExtensionRecord) *api.View {
	items := make([]api.Item, 0, len(records))
	for _, record := range records {
		item := api.Item{ID: record.ID, Title: record.Title, Subtitle: stateOf(record), Icon: "puzzle"}
		if !record.Enabled {
			review := api.Run("review", "Review and Enable", map[string]string{"id": record.ID})
			item.Primary = &review
		}
		items = append(items, item)
	}
	view := api.ListView("Extensions", items...)
	view.EmptyView = &api.EmptyState{Title: "No extensions.", Subtitle: "Installed extensions show here."}
	return &view
}

func stateOf(record api.ExtensionRecord) string {
	switch {
	case record.BuiltIn:
		return "Built in"
	case record.Enabled:
		return "On"
	}
	return "Off"
}
