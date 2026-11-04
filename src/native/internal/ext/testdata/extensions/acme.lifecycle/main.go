package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// notesPath is the indexed app every probe names; the host answers for it only when the index lists it.
const notesPath = "/Applications/Notes.app"

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
	running, err := ctx.Desktop().Apps().Running([]string{notesPath})
	if err != nil {
		return "running failed: " + err.Error()
	}
	return fmt.Sprintf("running=%d", len(running))
}

func quitCheck(ctx api.Context) string {
	result, err := ctx.Desktop().Apps().Quit(notesPath)
	if err != nil {
		return "quit failed: " + err.Error()
	}
	return fmt.Sprintf("quit was=%t quit=%t", result.WasRunning, result.Quit)
}

func hideCheck(ctx api.Context) string {
	if err := ctx.Desktop().Apps().Hide(notesPath); err != nil {
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
