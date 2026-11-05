package app

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
)

// previewTimeout bounds the reads that fill an extension's preview.
const previewTimeout = 5 * time.Second

// extensionPreviewView is the list an extension's preview shows: its root rows, then one row per command. A command row
// runs the command, and its view opens in the preview.
func extensionPreviewView(m *manager.Manager, extension string) api.View {
	ctx, cancel := context.WithTimeout(context.Background(), previewTimeout)
	defer cancel()
	view := api.View{Kind: api.ViewList, Title: extension, EmptyView: &api.EmptyState{Title: "Nothing to show yet", Subtitle: "Its rows appear once it runs."}}
	for _, result := range m.Roots(ctx) {
		if result.ExtensionID == extension {
			view.Title = result.Title
			view.Items = append(view.Items, result.Items...)
		}
	}
	for _, result := range m.Commands(ctx) {
		if result.ExtensionID == extension {
			view.Items = append(view.Items, commandPreviewItems(result.Commands)...)
		}
	}
	return view
}

// commandPreviewItems are the rows of an extension's commands in its preview: each runs its command.
func commandPreviewItems(commands []api.Command) []api.Item {
	items := make([]api.Item, 0, len(commands))
	for _, command := range commands {
		run := api.Run(command.ID, command.Title, nil)
		items = append(items, api.Item{ID: "command:" + command.ID, Title: command.Title, Subtitle: command.Subtitle, Icon: command.Icon, Primary: &run})
	}
	return items
}
