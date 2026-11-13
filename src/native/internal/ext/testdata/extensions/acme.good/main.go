package main

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

var words = []string{"alpha", "beta", "gamma"}

// Commands, RootItems, SearchItems and RunAction are the entry points the host calls. Their signatures are the
// api interfaces' methods, with the context as the first argument.
func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "hello", Title: "Say hello"}}, nil
}

func RootItems(ctx api.Context) ([]api.Item, error) {
	return []api.Item{{ID: "root", Title: "Good root"}}, nil
}

func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	var items []api.Item
	for _, word := range words {
		if strings.Contains(word, query) {
			items = append(items, api.Item{ID: word, Title: word})
		}
	}
	return items, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	return api.Result{Next: &api.Action{Kind: api.ActionRun, Title: "ran " + in.Ref.ID}}, nil
}
