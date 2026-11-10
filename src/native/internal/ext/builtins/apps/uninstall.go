package apps

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// The uninstall review: the app and the data it left, one toggle each, a confirmation, and a result. Discovery and the
// Trash run in the host, which keeps each review under its token; the selection is kept here.

// Titles and messages of the review, as 1.x words them.
const (
	reviewPlaceholder = "Filter items to move to Trash"
	reviewRowTitle    = "Review selected items"
	reviewRowSubtitle = "Confirm the items to move to Trash"
	noSelectionText   = "Select at least one item to move to Trash"
	quitAppNotice     = "If running, the application will be asked to quit first."
	unavailableTitle  = "Uninstall unavailable"
)

// startUninstall reviews the uninstall of one app and shows the review.
func (e Extension) startUninstall(ctx api.Context, path, name string) (api.Result, error) {
	found, err := ctx.Desktop().Apps().UninstallReview(path)
	if err != nil {
		return api.Result{}, err
	}
	if !found.Ready {
		view := api.ListView(unavailableTitle, api.Item{Title: unavailableTitle, Subtitle: found.Message, Icon: "x",
			Appearance: &api.Appearance{Foreground: api.ColorRed}})
		return ctx.Navigation().Push(&view), nil
	}
	r := &review{name: name, items: found.Items, notes: found.Notes, selected: map[string]bool{}}
	for _, item := range found.Items {
		r.selected[item.ID] = item.Selected
	}
	e.reviews.keep(found.Token, r)
	view := reviewView(ctx, found.Token, r)
	return ctx.Navigation().Push(&view), nil
}

// reviewView is the review list: the Review row, then one toggle per candidate, then the notes.
func reviewView(ctx api.Context, token string, r *review) api.View {
	view := api.ListView("Uninstall "+r.name, reviewItems(ctx, token, r)...)
	view.Placeholder = reviewPlaceholder
	return view
}

// reviewItems is the content of the review list, rebuilt from the selection after each toggle.
func reviewItems(ctx api.Context, token string, r *review) []api.Item {
	files := ctx.Desktop().Files()
	reviewRow := api.Run(refReview, "Review", map[string]string{argToken: token})
	items := []api.Item{{
		Title: reviewRowTitle, Subtitle: reviewRowSubtitle, Icon: "trash-2",
		Appearance: &api.Appearance{Foreground: api.ColorRed, Prominence: "primary"},
		Primary:    &reviewRow,
	}}
	for _, candidate := range r.items {
		toggle := api.Run(refToggle, "Toggle", map[string]string{argToken: token, argID: candidate.ID})
		items = append(items, api.Item{
			ID: candidate.ID, Title: files.DisplayPath(candidate.Path), Subtitle: candidate.Reason,
			Icon: candidateIcon(candidate.Kind), Accessories: []api.Accessory{selectionAccessory(r.selected[candidate.ID])},
			Primary: &toggle,
		})
	}
	for _, note := range r.notes {
		items = append(items, api.Item{Title: note, Icon: "info"})
	}
	return items
}

// candidateIcon is the row icon of a review item: the app bundle, or a folder of data.
func candidateIcon(kind string) string {
	if kind == "app" {
		return "app-window"
	}
	return "folder"
}

// selectionAccessory is the badge that says whether a review item is selected.
func selectionAccessory(selected bool) api.Accessory {
	if selected {
		return api.Accessory{Text: "Selected", Tone: api.ToneSuccess}
	}
	return api.Accessory{Text: "Optional", Tone: api.ToneMuted}
}

// runReview runs one step of the review: a toggle changes the selection, the review row asks for confirmation, and the
// confirmed Trash moves the selection and shows the result.
func (e Extension) runReview(ctx api.Context, ref string, args map[string]string) (api.Result, error) {
	token := args[argToken]
	switch ref {
	case refToggle:
		return e.toggleCandidate(ctx, token, args[argID])
	case refReview:
		return e.confirmSelection(ctx, token)
	}
	return e.trashSelection(ctx, token)
}

func (e Extension) toggleCandidate(ctx api.Context, token, id string) (api.Result, error) {
	r, ok := e.reviews.toggle(token, id)
	if !ok {
		return toast("This uninstall review has expired"), nil
	}
	return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: reviewItems(ctx, token, r)}}, nil
}

// confirmSelection asks before anything moves. An empty selection moves nothing and says so.
func (e Extension) confirmSelection(ctx api.Context, token string) (api.Result, error) {
	r, ok := e.reviews.lookup(token)
	if !ok {
		return toast("This uninstall review has expired"), nil
	}
	chosen := r.chosen()
	if len(chosen) == 0 {
		return toast(noSelectionText), nil
	}
	title, message := confirmText(ctx, chosen)
	trash := api.Run(refTrash, title, map[string]string{argToken: token})
	trash = api.WithConfirmation(trash, api.Confirmation{Title: title + "?", Message: message, ConfirmLabel: title, Destructive: true})
	return api.Result{Next: &trash}, nil
}

// confirmText is the question before the Trash run: the selected paths, and a line when the app itself will be quit.
func confirmText(ctx api.Context, chosen []api.UninstallItem) (title, message string) {
	files := ctx.Desktop().Files()
	noun := "items"
	if len(chosen) == 1 {
		noun = "item"
	}
	title = fmt.Sprintf("Move %d %s to Trash", len(chosen), noun)
	var lines []string
	quitsApp := false
	for _, item := range chosen {
		lines = append(lines, files.DisplayPath(item.Path))
		quitsApp = quitsApp || item.Kind == "app"
	}
	if quitsApp {
		lines = append(lines, quitAppNotice)
	}
	return title, joinLines(lines)
}

// trashSelection moves the selected items to the Trash and shows the result. The review is used up afterwards.
func (e Extension) trashSelection(ctx api.Context, token string) (api.Result, error) {
	r, ok := e.reviews.lookup(token)
	if !ok {
		return toast("This uninstall review has expired"), nil
	}
	ids := make([]string, 0)
	for _, item := range r.chosen() {
		ids = append(ids, item.ID)
	}
	result, err := ctx.Desktop().Apps().UninstallTrash(token, ids)
	if err != nil {
		return api.Result{}, err
	}
	e.reviews.drop(token)
	view := resultView(ctx, result)
	return ctx.Navigation().Push(&view), nil
}
