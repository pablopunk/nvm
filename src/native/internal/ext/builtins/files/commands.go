package files

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// The command IDs are the 1.x command IDs, so their shortcuts and aliases keep their keys.
const (
	searchCommandID  = "search-files"
	meaningCommandID = "find-files-by-meaning"
)

// Commands lists Search Files, and Find Files by Meaning while the account is signed in: the decision needs it.
func (Extension) Commands(ctx api.Context) ([]api.Command, error) {
	commands := []api.Command{{
		ID: searchCommandID, Title: searchTitle, Subtitle: "Find files in your home folder", Icon: "folder",
		Keywords: []string{"files", "find", "documents", "locate"}, Score: rootCommandScore, Appearance: &api.Appearance{Foreground: api.ColorBlue},
		Section: search.KindCommands, LearnKey: "cmd:" + searchTitle,
	}}
	if ctx.Account().SignedIn() {
		commands = append(commands, api.Command{
			ID: meaningCommandID, Title: meaningTitle, Subtitle: "Describe a file to find it with AI", Icon: "sparkles",
			Keywords: []string{"files", "find", "meaning", "describe", "semantic", "ai", "locate"}, Score: rootCommandScore,
			Section: search.KindCommands, LearnKey: "cmd:" + meaningTitle,
		})
	}
	return commands, nil
}

// rootCommandScore is the 1.x score of a command at the empty root, which is the default for commands.
const rootCommandScore = 12

// RunAction opens the Search Files or Find Files by Meaning view, and runs the view's own searches and asks.
func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case searchCommandID:
		view := e.searchFilesView(ctx)
		return ctx.Navigation().Push(&view), nil
	case meaningCommandID:
		view := e.meaningView(ctx)
		return ctx.Navigation().Push(&view), nil
	case refSearch:
		return e.searchChanged(ctx, in.Value), nil
	case refKind:
		return e.kindChanged(ctx, in.Value), nil
	case refStatus:
		return e.statusRefresh(ctx), nil
	case refMeaningSearch:
		return e.meaningSearchChanged(ctx, in.Value), nil
	case refMeaningAsk:
		return e.askMeaning(ctx, in.Ref.Args[argQuery]), nil
	}
	return api.Result{}, fmt.Errorf("files has no action %q", in.Ref.ID)
}
