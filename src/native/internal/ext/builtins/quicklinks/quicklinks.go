// Package quicklinks is the built-in that searches a website from a keyword: "gh nevermind" opens GitHub search for
// nevermind. It lists six defaults and the links the user creates, and it creates links from a form that the Create
// Quicklink command opens. The root does not list the command; a search finds it.
//
// Layer: ext. The package imports api, search and the standard library.
package quicklinks

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity.
const ID = "nevermind.quicklinks"

// CreateCommandID and CreateTitle name the command that opens the form. The native command kept both titles.
const (
	CreateCommandID = "create"
	CreateTitle     = "Create Quicklink"
)

// saveActionID is the form's submit action.
const saveActionID = "save"

// scoreQuicklink is the native band of a keyword row, above the commands and the web fallbacks.
const scoreQuicklink = 980

// storageKey is the key the links are kept under in the extension's storage.
const storageKey = "links"

// Link is one quicklink: a keyword that opens URL with the search text in place of {query}.
type Link struct {
	Name    string `json:"name"`
	Keyword string `json:"keyword"`
	URL     string `json:"url"`
}

// Extension is the quicklinks built-in. legacy holds the links the app kept in its settings file before this built-in;
// they answer searches until the user creates a link, which stores them with it.
type Extension struct {
	legacy []Link
}

// New returns the built-in with the links the settings file held.
func New(legacy []Link) Extension {
	return Extension{legacy: legacy}
}

// Manifest identifies the built-in. Its rows are listed under the Commands section, as the native rows were.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: ID, Title: "Quicklinks"}}
}

// Commands lists Create Quicklink. It is placed in search only, so the empty root does not show it.
func (Extension) Commands(api.Context) ([]api.Command, error) {
	return []api.Command{{
		ID: CreateCommandID, Title: CreateTitle, Icon: "link",
		Keywords:   []string{"quicklink", "bookmark", "url", "search", "new"},
		Appearance: &api.Appearance{Foreground: api.ColorBlue},
		Placement:  []api.Placement{api.PlacementSearch},
		Section:    search.KindCommands, LearnKey: "cmd:" + CreateTitle,
	}}, nil
}

// SearchItems answers "keyword search terms" with a search on the keyword's site, from the user's links first and then
// the defaults. A query with no search terms after its keyword matches nothing.
func (e Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	keyword, rest, ok := strings.Cut(query, " ")
	term := strings.TrimSpace(rest)
	if !ok || term == "" {
		return nil, nil
	}
	var out []api.Item
	for _, link := range append(e.links(ctx), defaultLinks...) {
		if strings.EqualFold(keyword, link.Keyword) {
			out = append(out, linkItem(link, term))
		}
	}
	return out, nil
}

// RunAction opens the form, or saves the link the form holds.
func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case CreateCommandID:
		view := createView()
		return ctx.Navigation().Push(&view), nil
	case saveActionID:
		return e.save(ctx, in.Form)
	}
	return api.Result{}, fmt.Errorf("the quicklinks built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// linkItem is the row of one link for term: it opens the link's site with the term as the search.
func linkItem(link Link, term string) api.Item {
	open := api.OpenURL("Open", searchURL(link, term))
	return api.Item{
		Title: fmt.Sprintf("Search %s for “%s”", link.Name, term), Subtitle: "Quicklink", Icon: "link",
		Section: search.KindQuicklinks, Score: scoreQuicklink,
		Appearance: &api.Appearance{Foreground: api.ColorBlue},
		Primary:    &open,
	}
}

// searchURL is the link's address with the term in place of {query}, escaped as a query value.
func searchURL(link Link, term string) string {
	return strings.ReplaceAll(link.URL, "{query}", url.QueryEscape(term))
}

// save validates the form's link and appends it to the stored links.
func (e Extension) save(ctx api.Context, form api.FormValues) (api.Result, error) {
	link := Link{
		Name:    strings.TrimSpace(form[fieldName].Text),
		Keyword: strings.TrimSpace(form[fieldKeyword].Text),
		URL:     strings.TrimSpace(form[fieldURL].Text),
	}
	if !isValidLink(link) {
		return api.Result{}, fmt.Errorf("a quicklink needs a name, a keyword and an http(s) URL: %w", api.ErrInvalidInput)
	}
	if err := ctx.Storage().Set(storageKey, append(e.links(ctx), link)); err != nil {
		return api.Result{}, err
	}
	return ctx.Navigation().Pop(), nil
}

// links are the stored links, or the legacy links while none is stored yet.
func (e Extension) links(ctx api.Context) []Link {
	var stored []Link
	if found, err := ctx.Storage().Get(storageKey, &stored); err == nil && found {
		return stored
	}
	return slices.Clone(e.legacy)
}

// isValidLink reports whether a link has every part it needs: a name, a keyword and an http or https address.
func isValidLink(link Link) bool {
	return link.Name != "" && link.Keyword != "" && strings.HasPrefix(link.URL, "http")
}
