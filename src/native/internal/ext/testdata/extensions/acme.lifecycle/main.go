package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// notesPath is the path of the indexed Notes app every probe names. The index supplies it, so it is absolute on every
// operating system, and the host answers for it only when the index lists it.
func notesPath(ctx api.Context) (string, error) {
	apps, err := ctx.Desktop().Apps().List()
	if err != nil {
		return "", err
	}
	for _, app := range apps {
		if app.Name == "Notes" {
			return app.Path, nil
		}
	}
	return "", fmt.Errorf("the index does not list Notes")
}

// checks maps each probe row to the lifecycle call it makes. Each row's title says what the host answered.
var checks = map[string]func(api.Context) string{
	"running":  runningCheck,
	"quit":     quitCheck,
	"hide":     hideCheck,
	"quit-all": quitAllCheck,
}

// SearchItems answers one probe row per query.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	check, ok := checks[query]
	if !ok {
		return nil, nil
	}
	return []api.Item{{ID: query, Title: check(ctx)}}, nil
}

func runningCheck(ctx api.Context) string {
	path, err := notesPath(ctx)
	if err != nil {
		return "running failed: " + err.Error()
	}
	running, err := ctx.Desktop().Apps().Running([]string{path})
	if err != nil {
		return "running failed: " + err.Error()
	}
	return fmt.Sprintf("running=%d", len(running))
}

func quitCheck(ctx api.Context) string {
	path, err := notesPath(ctx)
	if err != nil {
		return "quit failed: " + err.Error()
	}
	result, err := ctx.Desktop().Apps().Quit(path)
	if err != nil {
		return "quit failed: " + err.Error()
	}
	return fmt.Sprintf("quit was=%t quit=%t", result.WasRunning, result.Quit)
}

func hideCheck(ctx api.Context) string {
	path, err := notesPath(ctx)
	if err != nil {
		return "hide failed: " + err.Error()
	}
	if err := ctx.Desktop().Apps().Hide(path); err != nil {
		return "hide failed: " + err.Error()
	}
	return "hide sent=true"
}

func quitAllCheck(ctx api.Context) string {
	asked, err := ctx.Desktop().Apps().QuitAll()
	if err != nil {
		return "quit-all failed: " + err.Error()
	}
	return fmt.Sprintf("quit-all asked=%d", len(asked))
}
