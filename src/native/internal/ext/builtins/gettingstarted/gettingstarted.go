// Package gettingstarted is the built-in that welcomes a new user. Until the user dismisses it, the root lists a Welcome
// row that opens the guide; afterwards the row is Help. The guide lists the basics of the palette and the palette
// shortcut the user set, read live.
//
// Layer: ext. The package imports api, search and the standard library.
package gettingstarted

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity.
const ID = "nevermind.getting-started"

// The handlers of the Welcome row, its guide and the guide's done row.
const (
	openActionID    = "open"
	dismissActionID = "dismiss"
	finishActionID  = "finish"
)

// storageKey is the key the done flag is kept under in the extension's storage.
const storageKey = "done"

// guideKeywords are the words that find the Welcome row besides its title.
const guideKeywords = "help getting started welcome how to use commands"

// scoreAlias is the band of a command found by a keyword, as the native guide row scored it for a query.
const scoreAlias = 45

// welcomeScore is Welcome's score at the root, so it leads the commands; helpScore is Help's, below them.
const (
	welcomeScore = 90
	helpScore    = 14
)

// Extension is the Getting Started built-in. legacyDone is the done flag the settings file held before this built-in;
// it answers until the user dismisses the guide, which stores the flag.
type Extension struct {
	legacyDone bool
}

// New returns the built-in with the done flag the settings file held.
func New(legacyDone bool) Extension {
	return Extension{legacyDone: legacyDone}
}

// Manifest identifies the built-in and asks for the shortcut table, which the guide reads.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0", Identity: api.Identity{ID: ID, Title: "Getting Started"},
		Capabilities: []api.Capability{api.CapabilityShortcuts},
	}
}

// RootItems lists the Welcome row, or Help once the guide is done.
func (e Extension) RootItems(ctx api.Context) ([]api.Item, error) {
	return []api.Item{e.entry(ctx)}, nil
}

// SearchItems lists the Welcome row for a query its title or keywords match. The empty query is the root's.
func (e Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	lq := strings.ToLower(query)
	if lq == "" {
		return nil, nil
	}
	item := e.entry(ctx)
	if !matchesGuide(item.Title, lq) {
		return nil, nil
	}
	item.Score = scoreAlias
	return []api.Item{item}, nil
}

// RunAction opens the guide, dismisses the Welcome row, or finishes the guide.
func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case openActionID:
		view := guideView(ctx, e.completed(ctx))
		return ctx.Navigation().Push(&view), nil
	case dismissActionID:
		return api.Result{}, markComplete(ctx)
	case finishActionID:
		if err := markComplete(ctx); err != nil {
			return api.Result{}, err
		}
		return ctx.Navigation().Pop(), nil
	}
	return api.Result{}, fmt.Errorf("the getting started built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// entry is the Welcome row, or Help once the guide is done. Until then it offers to dismiss the welcome.
func (e Extension) entry(ctx api.Context) api.Item {
	open := api.Run(openActionID, "Get Started", nil)
	item := api.Item{
		Title: "Welcome to Nevermind", Subtitle: "Search apps and files, then press Enter. Launch the app to open the palette or set an optional shortcut.",
		Icon: "sparkles", Section: search.KindCommands, LearnKey: "guide", Score: welcomeScore, Primary: &open,
	}
	if e.completed(ctx) {
		help := api.Run(openActionID, "Open Help", nil)
		item.Title, item.Subtitle, item.Score, item.Primary = "Help", "Quick tips for finding and running commands", helpScore, &help
		return item
	}
	dismiss := api.Run(dismissActionID, "Dismiss welcome", nil)
	item.Actions = []api.Action{dismiss}
	return item
}

// completed reports whether the user is done with the guide: the stored flag, or the legacy one until it is stored.
func (e Extension) completed(ctx api.Context) bool {
	var done bool
	if found, err := ctx.Storage().Get(storageKey, &done); err == nil && found {
		return done
	}
	return e.legacyDone
}

// markComplete stores that the user is done with the guide, and runs the root again so the Welcome row becomes Help.
func markComplete(ctx api.Context) error {
	if err := ctx.Storage().Set(storageKey, true); err != nil {
		return err
	}
	ctx.Views().InvalidateRoot()
	return nil
}

// matchesGuide reports whether a lowercase query names the guide: it is part of the row's title, or of its keywords
// when it has two characters or more, as command aliases match.
func matchesGuide(title, lq string) bool {
	if strings.Contains(strings.ToLower(title), lq) {
		return true
	}
	return len(lq) >= 2 && strings.Contains(guideKeywords, lq)
}
