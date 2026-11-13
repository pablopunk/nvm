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

// RestartOp relaunches Nevermind without restarting the computer.
const RestartOp = "restart-app"

// QuitTitle is the title of the native command that quits the app, as 1.x titles it.
const QuitTitle = "Quit Nevermind"

// RestartTitle names the app restart command.
const RestartTitle = "Restart Nevermind"

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

// rootQuitScore is the 1.x score of Quit Nevermind at the empty root.
const rootQuitScore = 15

// RootItems lists Quit Nevermind at the empty root, in red as 1.x shows it.
func (Extension) RootItems(_ api.Context) ([]api.Item, error) {
	return []api.Item{quitItem(rootQuitScore)}, nil
}

// SearchItems lists Quit Nevermind when the query matches it. The empty root shows nothing.
func (Extension) SearchItems(_ api.Context, query string) ([]api.Item, error) {
	lq := strings.ToLower(query)
	if lq == "" {
		return nil, nil
	}
	var items []api.Item
	if score := commandmatch.Score(quitTitle, quitKeywords, lq); score > 0 {
		items = append(items, quitItem(score))
	}
	if strings.Contains(strings.ToLower(RestartTitle), lq) || strings.Contains("relaunch", lq) {
		if score := commandmatch.Score(RestartTitle, "restart relaunch nevermind", lq); score > 0 {
			items = append(items, restartItem(score))
		}
	}
	return items, nil
}

func restartItem(score int) api.Item {
	return api.Item{ID: RestartOp, Title: RestartTitle, Icon: "refresh-cw", Score: float64(score),
		Section: search.KindCommands, LearnKey: "cmd:" + RestartTitle,
		Primary: &api.Action{Kind: api.ActionSystem, Op: RestartOp, Title: "Restart"}}
}

// quitItem is the row of Quit Nevermind at score.
func quitItem(score int) api.Item {
	return api.Item{
		ID: QuitOp, Title: quitTitle, Icon: "x", Score: float64(score),
		Section: search.KindCommands, LearnKey: "cmd:" + quitTitle,
		Appearance: &api.Appearance{Foreground: api.ColorRed},
		Primary:    &api.Action{Kind: api.ActionSystem, Op: QuitOp, Title: "Open"},
	}
}
