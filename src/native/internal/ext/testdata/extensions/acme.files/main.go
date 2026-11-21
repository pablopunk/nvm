package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SearchItems answers one query per files method. Each row's title says what the host answered, so the supervisor
// test reads every method's outcome through the child process.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	check, ok := checks[query]
	if !ok {
		return nil, nil
	}
	return []api.Item{{ID: query, Title: check(ctx)}}, nil
}

var checks = map[string]func(api.Context) string{
	"recent":    recentCheck,
	"search":    searchCheck,
	"snapshot":  snapshotCheck,
	"roots":     rootsCheck,
	"reindex":   reindexCheck,
	"text":      textCheck,
	"write":     writeCheck,
	"quicklook": quickLookCheck,
	"pictures":  picturesCheck,
}

// notePath is the note the fixture reads and writes beside: under the first index root, which the test makes its own
// temporary folder.
func notePath(ctx api.Context, name string) string {
	return ctx.Desktop().Files().IndexedRoots()[0] + "/notes/" + name
}

func recentCheck(ctx api.Context) string {
	recent := ctx.Desktop().Files().Recent(api.IndexOptions{FindOptions: api.FindOptions{Limit: 6}})
	return fmt.Sprintf("recent count=%d first=%s", len(recent), firstName(recent))
}

func searchCheck(ctx api.Context) string {
	found := ctx.Desktop().Files().SearchIndex("todo", api.IndexOptions{})
	return fmt.Sprintf("search count=%d first=%s", len(found), firstName(found))
}

func snapshotCheck(ctx api.Context) string {
	snapshot := ctx.Desktop().Files().IndexSnapshot(api.IndexOptions{FindOptions: api.FindOptions{Kind: "image"}})
	return fmt.Sprintf("snapshot images=%d", len(snapshot))
}

func rootsCheck(ctx api.Context) string {
	return fmt.Sprintf("roots count=%d", len(ctx.Desktop().Files().IndexedRoots()))
}

func reindexCheck(ctx api.Context) string {
	report, err := ctx.Desktop().Files().Reindex(api.IndexOptions{})
	if err != nil {
		return "reindex refused=true"
	}
	return fmt.Sprintf("reindex count=%d", report.Count)
}

func textCheck(ctx api.Context) string {
	text, err := ctx.Desktop().Files().ReadText(notePath(ctx, "todo.txt"))
	if err != nil {
		return "text refused=true"
	}
	return "text " + text
}

func writeCheck(ctx api.Context) string {
	ref, err := ctx.Desktop().Files().WriteText(notePath(ctx, "out.txt"), "saved", false)
	if err != nil {
		return "write refused=true"
	}
	return "write " + ref.Name
}

func quickLookCheck(ctx api.Context) string {
	if err := ctx.Desktop().Files().QuickLook(notePath(ctx, "todo.txt")); err != nil {
		return "quicklook refused=true"
	}
	return "quicklook shown=true"
}

func picturesCheck(ctx api.Context) string {
	pictures, err := ctx.Desktop().Files().FindImages([]string{ctx.Desktop().Files().IndexedRoots()[0] + "/pics"}, api.FindOptions{Kind: "video"})
	if err != nil {
		return "pictures refused=true"
	}
	return fmt.Sprintf("pictures count=%d", len(pictures))
}

func firstName(files []api.IndexedFile) string {
	if len(files) == 0 {
		return "none"
	}
	return files[0].Name
}
