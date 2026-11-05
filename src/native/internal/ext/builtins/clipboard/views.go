package clipboard

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// listView is the Clipboard History list: the newest copies, a search over the whole history, and the removals of
// the history as a whole.
func (Extension) listView(ctx api.Context, options Options) api.View {
	return api.View{
		Kind: api.ViewList, Title: listTitle, Placeholder: listPlaceholder, Presentation: api.PresentSidePreview,
		Items:          copyItems(ctx, historyListLimit, options),
		EmptyView:      &api.EmptyState{Title: emptyTitle, Subtitle: emptySubtitle},
		Actions:        historyActions(),
		OnSearchChange: &api.ActionRef{ID: refSearch},
		Refresh:        &api.Refresh{Ref: &api.ActionRef{ID: refRefresh}},
	}
}

// historyActions are the removals of the whole history, listed on the list's own menu.
func historyActions() []api.Action {
	return []api.Action{
		api.Destructive(api.Run(refRemoveWindow, "Remove Entries from Last Hour", map[string]string{argWindow: windowHour})),
		api.Destructive(api.Run(refRemoveWindow, "Remove Entries from Last Day", map[string]string{argWindow: windowDay})),
		api.Destructive(api.Run(refRemoveAll, "Remove All Entries", nil)),
	}
}

// copyItems returns the rows of the newest copies, up to limit, with the rows' score left to the list.
func copyItems(ctx api.Context, limit int, options Options) []api.Item {
	entries, err := ctx.Clipboard().List(api.ClipboardQuery{Limit: limit})
	if err != nil {
		return nil
	}
	look := lookOf(ctx, options)
	items := make([]api.Item, 0, len(entries))
	for _, entry := range entries {
		items = append(items, copyRow(entry, 0, look))
	}
	return items
}

// previewEntry opens a copy full-palette, as the Preview action of 1.x does: its text, or its picture at preview size.
func previewEntry(ctx api.Context, id string) (api.Result, error) {
	entry, found, err := ctx.Clipboard().Get(id)
	if err != nil {
		return api.Result{}, err
	}
	if !found {
		return api.Result{Toast: &api.Toast{Message: missingMessage, Tone: "error"}}, nil
	}
	view := api.View{Kind: api.ViewPreview, Title: titleOf(entry), Preview: previewSpec(entry)}
	return ctx.Navigation().Push(&view), nil
}

// previewSpec is the content of the full preview: the picture when the copy has one, otherwise its text.
func previewSpec(entry api.ClipboardEntry) *api.PreviewSpec {
	if entry.Type == kindImage {
		return &api.PreviewSpec{Image: &api.Image{Src: entry.ImageDataURL, Fit: api.FitContain}}
	}
	return &api.PreviewSpec{Text: entry.Text}
}

// searchResult answers the list's search: the copies whose text contains the query, as a second batch of rows.
func searchResult(ctx api.Context, query string, options Options) (api.Result, error) {
	if query == "" {
		return api.Result{}, nil
	}
	entries, err := ctx.Clipboard().List(api.ClipboardQuery{Text: query, Limit: searchPatchLimit})
	if err != nil {
		return api.Result{}, err
	}
	items := make([]api.Item, 0, len(entries))
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		row := copyRow(entry, 0, lookOf(ctx, options))
		items = append(items, row)
		ids = append(ids, row.ID)
	}
	return api.Result{Patch: &api.ViewPatch{Mode: api.PatchMerge, Items: items, SearchResultIds: ids}}, nil
}

// refreshResult replaces the list's rows with the newest copies, so a copy made while the list is open shows.
func refreshResult(ctx api.Context, options Options) api.Result {
	return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: copyItems(ctx, historyListLimit, options)}}
}

// removeLastView is the list the "Remove Last..." action opens: one confirmed removal per time window.
func removeLastView() api.View {
	windows := []struct{ window, title string }{
		{window5Minutes, "5 Minutes"}, {windowHour, "Hour"}, {windowDay, "Day"}, {windowWeek, "Week"},
	}
	items := make([]api.Item, 0, len(windows))
	for _, w := range windows {
		remove := api.Destructive(api.Run(refRemoveWindow, w.title, map[string]string{argWindow: w.window}))
		items = append(items, api.Item{ID: "window:" + w.window, Title: w.title, Icon: "clipboard", Primary: &remove})
	}
	return api.View{Kind: api.ViewList, Title: "Remove Last...", Placeholder: "Remove Last...", Items: items}
}
