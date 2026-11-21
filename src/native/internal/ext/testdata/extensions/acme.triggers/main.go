package main

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.triggers is the dev fixture of triggers. "files" runs on files.changed in ~/nvm-j18-watch, debounced, and
// "clipboard" runs on clipboard.changed. Each run adds one to its own count in storage, and a search row shows both
// counts, so the rig reads how often the triggers fired.

const watchRoot = "~/nvm-j18-watch"

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{
		{ID: "files", Title: "Count file changes", Triggers: []api.Trigger{{Type: api.TriggerFilesChanged, Roots: []string{watchRoot}, DebounceMs: 800}}},
		{ID: "clipboard", Title: "Count clipboard copies", Triggers: []api.Trigger{{Type: api.TriggerClipboardChanged}}},
	}, nil
}

func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if !strings.Contains("trigger", strings.ToLower(query)) {
		return nil, nil
	}
	files, err := readCount(ctx, "files")
	if err != nil {
		return nil, err
	}
	clipboard, err := readCount(ctx, "clipboard")
	if err != nil {
		return nil, err
	}
	return []api.Item{{ID: "counts", Title: fmt.Sprintf("Trigger counts: files %d, clipboard %d", files, clipboard)}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	count, err := readCount(ctx, in.Ref.ID)
	if err != nil {
		return api.Result{}, err
	}
	if err := ctx.Storage().Set(in.Ref.ID, count+1); err != nil {
		return api.Result{}, err
	}
	return api.Result{}, nil
}

func readCount(ctx api.Context, key string) (int, error) {
	var count int
	_, err := ctx.Storage().Get(key, &count)
	return count, err
}
