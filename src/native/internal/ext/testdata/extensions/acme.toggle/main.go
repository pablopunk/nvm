package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "open", Title: "Toggle action fixture"}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID == "read" {
		var enabled bool
		if err := ctx.Settings().Get("compact", &enabled); err != nil {
			return api.Result{}, err
		}
		title := "Compact disabled"
		if enabled {
			title = "Compact enabled"
		}
		view := api.ListView("Current value", api.Item{ID: "value", Title: title})
		return api.Result{View: &view}, nil
	}
	toggle := api.Action{Kind: api.ActionToggleSetting, Title: "Toggle", Setting: "compact", Confirm: &api.Confirmation{Title: "Toggle compact?", ConfirmLabel: "Toggle"}}
	view := api.ListView("Toggle action fixture",
		api.Item{ID: "toggle", Title: "Toggle compact", Primary: api.Primary(toggle)},
		api.Item{ID: "read", Title: "Inspect current value", Primary: api.Primary(api.Run("read", "Inspect", nil))},
	)
	return api.Result{View: &view}, nil
}
