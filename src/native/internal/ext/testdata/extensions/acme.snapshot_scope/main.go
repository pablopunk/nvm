package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	_, _ = ctx.Desktop().Clipboard().ReadText()
	_, _ = ctx.Network().Fetch(api.FetchRequest{URL: "http://127.0.0.1:9/"})
	if in.Ref.Args["probe"] != "" {
		_, _ = ctx.Desktop().Files().Thumbnail(in.Ref.Args["probe"])
	}
	thumbnail, found := ctx.Desktop().Files().Thumbnail(in.Ref.Args["path"])
	item := api.Item{ID: in.Ref.Args["path"], Title: "Handled unrelated denials"}
	if found {
		item.Image = &api.Image{Src: thumbnail}
	}
	view := api.GridView("Scope", 0, item)
	return api.Result{View: &view}, nil
}
