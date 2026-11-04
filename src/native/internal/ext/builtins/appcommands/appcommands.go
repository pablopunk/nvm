// Package appcommands is the built-in that finds the commands of the app itself. It holds Quit Nevermind, which the
// root lists at the place the native command held.
//
// Layer: ext. The package imports api, search, commandmatch and the standard library.
package appcommands

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/commandmatch"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity. The app lists its rows by it at the place the native commands held.
const ID = "nevermind.app-commands"

// QuitOp is the system operation that quits the app; the host runs it.
const QuitOp = "quit"

// QuitTitle is the title of the native command that quits the app, as 1.x titles it.
const QuitTitle = "Quit Nevermind"

// quitTitle and quitKeywords are the native command's, unchanged, so the root finds it as it always did.
const (
	quitTitle    = QuitTitle
	quitKeywords = "quit exit nevermind"
)

// Extension is the app commands built-in.
type Extension struct{}

// Manifest identifies the built-in. Its rows are listed under the Commands section, as the native rows were.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: ID, Title: "App Commands"}}
}

// SearchItems lists Quit Nevermind when the query matches it. The empty root shows nothing.
func (Extension) SearchItems(_ api.Context, query string) ([]api.Item, error) {
	lq := strings.ToLower(query)
	if lq == "" {
		return nil, nil
	}
	score := commandmatch.Score(quitTitle, quitKeywords, lq)
	if score == 0 {
		return nil, nil
	}
	return []api.Item{{
		ID: QuitOp, Title: quitTitle, Icon: "x", Score: float64(score),
		Section: search.KindCommands, LearnKey: "cmd:" + quitTitle,
		Primary: &api.Action{Kind: api.ActionSystem, Op: QuitOp, Title: "Open"},
	}}, nil
}
