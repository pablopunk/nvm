// Package places is the built-in that finds the standard folders of the home folder (Downloads, Documents, Desktop,
// Pictures) by name or keyword and opens them in the file manager. The root lists its rows at the place the native
// folder commands held.
//
// Layer: ext. The package imports api, commandmatch and the standard library.
package places

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/commandmatch"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity. The app asks for its rows by it, at the place the folder commands held.
const ID = "nevermind.places"

// placeKeywords are the native folder commands' keywords, unchanged, so a query for "open" or "go" finds them as it did.
const placeKeywords = "open folder places go"

// placeIcon is the tile of every folder row.
const placeIcon = "folder"

// Extension is the places built-in.
type Extension struct{}

// Manifest identifies the built-in. Its rows are listed under the Commands section, as the native rows were.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: ID, Title: "Places"}}
}

// SearchItems lists the folders whose names, or keywords, match the query. Folders show only for a query, never at the
// empty root.
func (Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	lq := strings.ToLower(query)
	if lq == "" {
		return nil, nil
	}
	var out []api.Item
	for _, place := range ctx.System().Places() {
		if score := commandmatch.Score(place.Name, placeKeywords, lq); score > 0 {
			out = append(out, placeItem(place, score))
		}
	}
	return out, nil
}

// placeItem is the row of one folder: its name, its path as the subtitle, and an Open action that opens it.
func placeItem(place api.Place, score int) api.Item {
	return api.Item{
		Title: place.Name, Subtitle: place.Path, Icon: placeIcon,
		Score: float64(score), Section: search.KindCommands, LearnKey: "cmd:" + place.Name,
		Appearance: &api.Appearance{Foreground: api.ColorBlue},
		Primary:    primaryAction(place.Path),
	}
}

// primaryAction opens the folder when the row runs: the host hides the palette and hands the path to the file manager.
func primaryAction(path string) *api.Action {
	action := api.OpenPath("Open", path)
	return &action
}
