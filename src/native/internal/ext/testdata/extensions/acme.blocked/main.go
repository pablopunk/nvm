package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// RootItems waits for a host shell call. The wait is host time, so a root budget far shorter than the call does not
// fail the extension.
func RootItems(ctx api.Context) ([]api.Item, error) {
	result, err := ctx.Desktop().Shell().Exec("slow-tool", nil, api.ShellOptions{})
	if err != nil {
		return nil, err
	}
	return []api.Item{{ID: "done", Title: "Shell finished: " + result.Stdout}}, nil
}
