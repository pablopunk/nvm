package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "navigation", Title: "Navigation fixture"}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	replaced := api.ListView("Replacement", api.Item{ID: "replacement", Title: "Replacement destination"})
	root := api.ListView("New root", api.Item{ID: "root", Title: "Root destination"})
	branch := api.ListView("Branch",
		api.Item{ID: "replace", Title: "Replace destination", Primary: api.Primary(api.Action{Kind: api.ActionReplaceView, Title: "Replace", View: &replaced})},
		api.Item{ID: "root", Title: "Go to root destination", Primary: api.Primary(api.RootView("Root", root))},
	)
	view := api.ListView("Navigation", api.Item{ID: "branch", Title: "Open branch", Primary: api.Primary(api.PushView("Open", branch))})
	return api.Result{View: &view}, nil
}
