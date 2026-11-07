package extview

import (
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// WithPreviewActions gives an item the generic preview options 1.x adds to every row with preview content and no preview
// of its own: Preview shows the content in the palette, and Preview File, for a file, hands it to the system previewer. Built-in
// and folder extension rows both pass through here, so both get the same options. The item is returned unchanged when it
// has nothing to preview or already offers a preview.
func WithPreviewActions(item api.Item) api.Item {
	generic := previewActions(item)
	if len(generic) == 0 {
		return item
	}
	if item.ActionPanel != nil && len(item.ActionPanel.Sections) > 0 {
		panel := *item.ActionPanel
		panel.Sections = append(slices.Clip(panel.Sections), api.ActionSection{Actions: generic})
		item.ActionPanel = &panel
		return item
	}
	item.Actions = append(slices.Clip(item.Actions), generic...)
	return item
}

// previewActions are the generic preview options of item, or none when item has no preview content of its own or already
// offers a preview.
func previewActions(item api.Item) []api.Action {
	if !hasPreviewContent(item) || offersPreview(item) {
		return nil
	}
	file := previewFileOf(item)
	preview := api.Action{Kind: api.ActionPushView, Title: api.PreviewTitle, Shortcut: api.PreviewShortcut, View: previewViewOf(item, file), Preview: true}
	if file == nil {
		return []api.Action{preview}
	}
	quickLook := api.QuickLook(api.PreviewFileTitle, file.Path)
	quickLook.Shortcut, quickLook.Preview = api.PreviewShortcut, true
	return []api.Action{preview, quickLook}
}

// previewViewOf is the palette view Preview pushes: the file's own preview when item previews a file, else the item's text,
// picture and video.
func previewViewOf(item api.Item, file *api.FileRef) *api.View {
	if file != nil {
		view := api.PreviewFileView(*file)
		return &view
	}
	spec := &api.PreviewSpec{Image: previewImageOf(item), Video: previewVideoOf(item)}
	if item.Detail != nil {
		spec.Markdown, spec.Text = item.Detail.Markdown, item.Detail.Text
	}
	return &api.View{Kind: api.ViewPreview, Title: item.Title, Preview: spec}
}

// hasPreviewContent is true when item carries a picture, a video, text or a file to show. A row's File alone does not
// count: it is the file a row drags out and whose icon it shows, as an application row's bundle is.
func hasPreviewContent(item api.Item) bool {
	if previewFileOf(item) != nil || previewImageOf(item) != nil || previewVideoOf(item) != "" {
		return true
	}
	return item.Detail != nil && (item.Detail.Text != "" || item.Detail.Markdown != "")
}

// offersPreview is true when an action of item already previews its content, so the generic option would repeat it.
func offersPreview(item api.Item) bool {
	actions := append([]api.Action{}, item.Actions...)
	if item.Primary != nil {
		actions = append(actions, *item.Primary)
	}
	if item.ActionPanel != nil {
		for _, section := range item.ActionPanel.Sections {
			actions = append(actions, section.Actions...)
		}
	}
	return slices.ContainsFunc(actions, isPreviewAction)
}

// isPreviewAction is true for an action that previews content: one the extension or the host marks as its preview, or
// one that pushes a preview view.
func isPreviewAction(action api.Action) bool {
	if action.Preview {
		return true
	}
	return action.Kind == api.ActionPushView && action.View != nil &&
		(action.View.Kind == api.ViewPreview || action.View.Kind == api.ViewFilePreview)
}

// previewFileOf is the file item previews: the file of its detail pane.
func previewFileOf(item api.Item) *api.FileRef {
	if item.Detail != nil {
		return item.Detail.File
	}
	return nil
}

// previewImageOf is the picture item shows: its Image, or the image of its detail pane.
func previewImageOf(item api.Item) *api.Image {
	if item.Image != nil {
		return item.Image
	}
	if item.Detail != nil {
		return item.Detail.Image
	}
	return nil
}

// previewVideoOf is the video item shows: its Video, or the video of its detail pane.
func previewVideoOf(item api.Item) string {
	if item.Video != "" {
		return item.Video
	}
	if item.Detail != nil {
		return item.Detail.Video
	}
	return ""
}
