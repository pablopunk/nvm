package apps

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// The result of a Trash run. A complete run is one line; a partial or failed run lists what stayed in place, with the
// reason for each, and what moved. Reveal in Trash opens the folder the items went to.
const (
	resultTitle          = "Uninstall"
	resultPartialTitle   = "Uninstall partially completed"
	resultFailedTitle    = "Nothing was moved to Trash"
	resultPartialMessage = "Some selected items were left untouched."
	resultFailedMessage  = "Selected items were left untouched."
	movedToTrash         = "Moved to Trash"
	revealTrashTitle     = "Reveal in Trash"
	revealTrashSubtitle  = "Show the Trash folder"
	statusComplete       = "complete"
	statusPartial        = "partial"
)

// resultView shows what a Trash run moved and what it left.
func resultView(ctx api.Context, result api.UninstallResult) api.View {
	files := ctx.Desktop().Files()
	var items []api.Item
	if result.Status == statusComplete {
		items = append(items, api.Item{Title: movedSummary(len(result.Moved)), Icon: "check", Appearance: &api.Appearance{Foreground: api.ColorGreen}})
	} else {
		items = append(items, api.Item{Title: failureMessage(result.Status), Icon: "info"})
		for _, untouched := range result.Untouched {
			items = append(items, api.Item{Title: files.DisplayPath(untouched.Path), Subtitle: untouched.Message, Icon: "file"})
		}
		for _, moved := range result.Moved {
			items = append(items, api.Item{Title: files.DisplayPath(moved), Subtitle: movedToTrash, Icon: "file"})
		}
	}
	reveal := api.RevealPath(revealTrashTitle, result.TrashFolder)
	items = append(items, api.Item{Title: revealTrashTitle, Subtitle: revealTrashSubtitle, Icon: "trash-2", Primary: &reveal})
	return api.ListView(resultTitleOf(result.Status), items...)
}

func resultTitleOf(status string) string {
	switch status {
	case statusComplete:
		return resultTitle
	case statusPartial:
		return resultPartialTitle
	}
	return resultFailedTitle
}

func failureMessage(status string) string {
	if status == statusPartial {
		return resultPartialMessage
	}
	return resultFailedMessage
}

func movedSummary(count int) string {
	noun := "items"
	if count == 1 {
		noun = "item"
	}
	return fmt.Sprintf("%d %s moved to Trash", count, noun)
}
