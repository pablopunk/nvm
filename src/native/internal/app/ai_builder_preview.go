package app

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
)

// previewTimeout bounds the reads that fill an extension's preview.
const previewTimeout = 5 * time.Second

// extensionPreview is the preview of one extension beside its builder chats: the extview session that holds the
// extension's rows and commands, and the side that shows them.
type extensionPreview struct {
	session *extview.Session
	side    *palette.Side
}

// previewSide is the side that shows an extension beside its builder chat, or nil until the extension was read.
func (p *App) previewSide(extension string) *palette.Side {
	if preview := p.previews[extension]; preview != nil {
		return preview.side
	}
	return nil
}

// refreshPreview reads what an extension shows now, its root rows and its commands, off the UI thread, and shows the
// result on the UI thread. It runs after each write, so the preview follows the extension.
func (p *App) refreshPreview(extension string) {
	m := p.extensionManager()
	if m == nil {
		return
	}
	go func() {
		view := extensionPreviewView(m, extension)
		p.onUI(func() { p.showPreview(m, extension, view) })
	}()
}

// showPreview keeps an extension's preview. The first read creates its session and side; later reads replace the
// session's list and search the side again.
func (p *App) showPreview(m *manager.Manager, extension string, view api.View) {
	if preview := p.previews[extension]; preview != nil {
		preview.session.Replace(view)
		preview.side.Refresh()
		return
	}
	if p.previews == nil {
		p.previews = map[string]*extensionPreview{}
	}
	session := extview.NewSession(p.extensionViewDeps(m), extension, view)
	root := session.Palette()
	root.HideInput = true
	p.previews[extension] = &extensionPreview{session: session, side: palette.NewSide(root)}
}

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
