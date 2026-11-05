// Package snippets is the built-in that keeps named pieces of text the user pastes often. A keyword typed at the root
// finds a snippet, the Snippets command lists them, and Enter pastes the text into the app that had focus after its
// {clipboard} and {date} placeholders are filled. Its form creates and edits snippets, and a delete asks first.
//
// Layer: ext. The package imports api, search and the standard library.
package snippets

import (
	"fmt"
	"sync/atomic"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity.
const ID = "nevermind.snippets"

// ListCommandID and CreateCommandID name the commands: the list of snippets, and the form that creates one.
const (
	ListCommandID   = "snippets"
	CreateCommandID = "create"
)

// Titles the commands and the form keep from the native rows.
const (
	listTitle   = "Snippets"
	createTitle = "Create Snippet"
	editTitle   = "Edit Snippet"
)

// Refs of the actions the rows, the views and the forms run. The args carry the snippet's id.
const (
	refPaste      = "paste"
	refCopy       = "copy"
	refEdit       = "edit"
	refDelete     = "delete"
	refSave       = "save"
	refCreate     = "create-form"
	refRefreshAll = "refresh-list"
	argID         = "id"
)

// Scores of a keyword row: an exact keyword ranks above a prefix, and both above the commands.
const (
	scoreSnippetExact  = 975
	scoreSnippetPrefix = 970
)

// scoreRootCommand is the root score of a command the native root listed with no score of its own. The Snippets row
// keeps it, so its place among the root commands does not move.
const scoreRootCommand = 12

// pasteTitle is the title of the paste action. The palette's feedback names a paste by it.
const pasteTitle = "Paste"

// Extension is the snippets built-in. legacy holds the snippets the app kept in its snippets document before this
// built-in; they answer until the first save, which stores them with the new one. sequence keeps snippet ids apart when
// two are made in the same nanosecond.
type Extension struct {
	legacy   []Snippet
	sequence *atomic.Uint64
}

// New returns the snippets built-in with the snippets the legacy document held.
func New(legacy []Snippet) Extension {
	return Extension{legacy: legacy, sequence: new(atomic.Uint64)}
}

// Manifest identifies the built-in. Its rows are listed under the Commands and Snippets sections, as the native rows were.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: listTitle},
		Capabilities: []api.Capability{api.CapabilityDesktopClipboard},
	}
}

// Commands lists Snippets at the root and Create Snippet in search only, as the native commands were.
func (Extension) Commands(api.Context) ([]api.Command, error) {
	return []api.Command{
		{
			ID: ListCommandID, Title: listTitle, Subtitle: "Saved text you paste often", Icon: "type", Score: scoreRootCommand,
			Keywords: []string{"snippets", "text", "expand", "template"}, Appearance: &api.Appearance{Foreground: api.ColorTeal},
			Section: search.KindCommands, LearnKey: "cmd:" + listTitle, Customizable: true,
		},
		{
			ID: CreateCommandID, Title: createTitle, Icon: "plus",
			Keywords: []string{"snippet", "new", "add"}, Appearance: &api.Appearance{Foreground: api.ColorTeal},
			Placement: []api.Placement{api.PlacementSearch}, Section: search.KindCommands, LearnKey: "cmd:" + createTitle,
		},
	}, nil
}

// SearchItems answers a query with the snippets whose keyword is the query (ranked first) or starts with it. A snippet
// with no keyword is found by the Snippets command only.
func (e Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	exact, prefix := matchKeywords(e.listSnippets(ctx), query)
	canPaste := pasteAvailable(ctx)
	var out []api.Item
	for _, sn := range exact {
		out = append(out, snippetRow(sn, scoreSnippetExact, canPaste))
	}
	for _, sn := range prefix {
		out = append(out, snippetRow(sn, scoreSnippetPrefix, canPaste))
	}
	return out, nil
}

// RunAction runs the commands, the rows' actions, the form's save and the delete confirmation.
func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	args := in.Ref.Args
	switch in.Ref.ID {
	case ListCommandID:
		return pushView(ctx, listView(e.listSnippets(ctx), pasteAvailable(ctx), e.refresh())), nil
	case CreateCommandID, refCreate:
		return pushView(ctx, formView(createTitle, Snippet{}, api.Run(refSave, "Save", nil))), nil
	case refPaste:
		return e.paste(ctx, args[argID])
	case refCopy:
		return e.copy(ctx, args[argID])
	case refEdit:
		return e.edit(ctx, args[argID])
	case refDelete:
		return e.delete(ctx, args[argID])
	case refSave:
		return e.save(ctx, args[argID], in.Form)
	case refRefreshAll:
		return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: listItems(e.listSnippets(ctx), pasteAvailable(ctx))}}, nil
	}
	return api.Result{}, fmt.Errorf("the snippets built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// refresh is the list view's refresh action: it re-reads the snippets, so a save shows up without a new search.
func (Extension) refresh() *api.Refresh {
	return &api.Refresh{Ref: &api.ActionRef{ID: refRefreshAll}}
}

// paste fills the placeholders of a snippet and pastes it through the paste family.
func (e Extension) paste(ctx api.Context, id string) (api.Result, error) {
	sn, err := e.snippetByID(ctx, id)
	if err != nil {
		return api.Result{}, err
	}
	next := api.PasteText(pasteTitle, expanded(ctx, sn))
	return api.Result{Next: &next}, nil
}

// copy puts the filled text on the clipboard without pasting it, and closes the palette.
func (e Extension) copy(ctx api.Context, id string) (api.Result, error) {
	sn, err := e.snippetByID(ctx, id)
	if err != nil {
		return api.Result{}, err
	}
	return api.Result{}, ctx.Desktop().Clipboard().WriteText(expanded(ctx, sn), false)
}

// edit opens the form with the snippet's fields.
func (e Extension) edit(ctx api.Context, id string) (api.Result, error) {
	sn, err := e.snippetByID(ctx, id)
	if err != nil {
		return api.Result{}, err
	}
	return pushView(ctx, formView(editTitle, sn, api.Run(refSave, "Save", map[string]string{argID: id}))), nil
}

// delete removes the snippet the user confirmed, and shows the list without it.
func (e Extension) delete(ctx api.Context, id string) (api.Result, error) {
	if err := e.deleteSnippet(ctx, id); err != nil {
		return api.Result{}, err
	}
	return api.Result{
		Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: listItems(e.listSnippets(ctx), pasteAvailable(ctx))},
		Toast: &api.Toast{Message: "Snippet deleted", Tone: "success"},
	}, nil
}

// save creates the snippet the form holds, or updates the one named by id, and returns to the list, which refreshes.
func (e Extension) save(ctx api.Context, id string, form api.FormValues) (api.Result, error) {
	input := Input{Name: form[fieldName].Text, Keyword: form[fieldKeyword].Text, Text: form[fieldContent].Text}
	var err error
	if id == "" {
		_, err = e.createSnippet(ctx, input)
	} else {
		_, err = e.updateSnippet(ctx, id, input)
	}
	if err != nil {
		return api.Result{}, err
	}
	return api.Result{Navigation: api.NavPop}, nil
}

// pushView opens view above the current page.
func pushView(ctx api.Context, view api.View) api.Result {
	return ctx.Navigation().Push(&view)
}

// snippetByID returns the snippet with id, or says it no longer exists.
func (e Extension) snippetByID(ctx api.Context, id string) (Snippet, error) {
	sn, ok := findSnippet(e.listSnippets(ctx), id)
	if !ok {
		return Snippet{}, errNotFound
	}
	return sn, nil
}

// pasteAvailable reports whether the platform can send the paste gesture. Where it cannot, a snippet's first action copies.
func pasteAvailable(ctx api.Context) bool {
	return ctx.System().HasCapability("clipboard.paste")
}

// expanded is the snippet's text with its placeholders filled from the clipboard and today's date.
func expanded(ctx api.Context, sn Snippet) string {
	clip, _ := ctx.Desktop().Clipboard().ReadText()
	return Expand(sn.Text, clip, ctx.Clock().Now())
}
