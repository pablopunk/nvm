package clipboard

import (
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Removal windows, named as the actions carry them.
const (
	window5Minutes = "5m"
	windowHour     = "1h"
	windowDay      = "1d"
	windowWeek     = "1w"
	windowAll      = "all"
)

// windowLength is how far back each removal window reaches.
var windowLength = map[string]time.Duration{
	window5Minutes: 5 * time.Minute, windowHour: time.Hour, windowDay: 24 * time.Hour, windowWeek: 7 * 24 * time.Hour,
}

// Messages the actions show.
const (
	missingMessage = "That item is no longer in clipboard history."
	copyFailed     = "Copy failed."
)

// maxRemoveBatch is the most entry ids one removal call takes; the service refuses more.
const maxRemoveBatch = 200

// maxScanEntries is the most copies a window removal reads, the most a list call returns.
const maxScanEntries = 500

// removeLastAction opens the "Remove Last..." list of time windows on a row's menu.
func removeLastAction() api.Action {
	return api.Run(refRemoveLast, "Remove Last...", nil)
}

// pushRemoveLast opens the list of time windows the "Remove Last..." action names.
func pushRemoveLast(ctx api.Context) api.Result {
	view := removeLastView()
	return ctx.Navigation().Push(&view)
}

// copyEntry puts a copy back on the clipboard, keeping its formats: markup for HTML, the file list for files. Plain
// text and pictures are copied by the row's own actions.
func copyEntry(ctx api.Context, id string) (api.Result, error) {
	entry, found, err := ctx.Clipboard().Get(id)
	if err != nil {
		return api.Result{}, err
	}
	if !found {
		return api.Result{Toast: &api.Toast{Message: missingMessage, Tone: "error"}}, nil
	}
	content := api.ClipboardContent{Type: "text", Text: entry.Text}
	switch entry.Type {
	case kindHTML:
		content = api.ClipboardContent{Type: "html", HTML: entry.HTML, Text: entry.Text}
	case kindFiles:
		content = api.ClipboardContent{Type: "files", Paths: entry.FilePaths}
	}
	if err := ctx.Desktop().Clipboard().Write(content); err != nil {
		return api.Result{Toast: &api.Toast{Message: copyFailed, Tone: "error"}}, nil
	}
	return api.Result{}, nil
}

// removeEntry removes one copy and shows the list without it.
func removeEntry(ctx api.Context, id string) (api.Result, error) {
	removed, err := ctx.Clipboard().Remove([]string{id})
	if err != nil {
		return api.Result{}, err
	}
	return removedResult(ctx, removed), nil
}

// removeWindow removes the copies made inside a removal window, in batches the service accepts.
func removeWindow(ctx api.Context, window string) (api.Result, error) {
	if window == windowAll {
		return removeAll(ctx)
	}
	length, ok := windowLength[window]
	if !ok {
		return api.Result{}, fmt.Errorf("%q is not a removal window", window)
	}
	cutoff := ctx.Clock().Now().Add(-length).UnixMilli()
	entries, err := ctx.Clipboard().List(api.ClipboardQuery{Limit: maxScanEntries})
	if err != nil {
		return api.Result{}, err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.CreatedAtMs >= cutoff {
			ids = append(ids, entry.ID)
		}
	}
	removed, err := removeInBatches(ctx, ids)
	if err != nil {
		return api.Result{}, err
	}
	return removedResult(ctx, removed), nil
}

// removeInBatches removes ids in calls of at most maxRemoveBatch, and counts what went.
func removeInBatches(ctx api.Context, ids []string) (int, error) {
	total := 0
	for start := 0; start < len(ids); start += maxRemoveBatch {
		end := min(start+maxRemoveBatch, len(ids))
		removed, err := ctx.Clipboard().Remove(ids[start:end])
		if err != nil {
			return total, err
		}
		total += removed
	}
	return total, nil
}

// removeAll clears the whole history.
func removeAll(ctx api.Context) (api.Result, error) {
	removed, err := ctx.Clipboard().Clear(nil, 0)
	if err != nil {
		return api.Result{}, err
	}
	return removedResult(ctx, removed), nil
}

// removedResult says how many copies went, and refreshes the list and the root so they no longer show them.
func removedResult(ctx api.Context, removed int) api.Result {
	refresh := ctx.Views().Refresh()
	ctx.Views().InvalidateRoot()
	return api.Result{Next: &refresh, Toast: &api.Toast{Message: removedMessage(removed), Tone: "success"}}
}

// removedMessage is the toast after a removal.
func removedMessage(n int) string {
	if n == 1 {
		return "Removed 1 clipboard item."
	}
	return fmt.Sprintf("Removed %d clipboard items.", n)
}
