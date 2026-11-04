package design

import "github.com/egoist/mygo/ui"

// galleryStates shows the states a view can be in: empty, a form with a focused field and an error, a chat
// bubble, and the pane itself.
func galleryStates(c *ui.Context) {
	Section(c, "EmptyState: nothing to show, and how to fix it")
	ui.Column(c).Height(120).Children(func() {
		EmptyState(c, EmptyData{Title: "No clipboard items found.", Subtitle: "Copy text or images and they will appear here."})
	})
	Section(c, "FormField: focused single line, and a multi-line editor")
	FormField(c, FieldData{Label: "Name", Placeholder: "Email signature", Value: &galleryState.single, Focus: true})
	ui.Column(c).Height(110).Children(func() {
		FormField(c, FieldData{Label: "Content", Placeholder: "Text to paste…", Value: &galleryState.multiline, Multiline: true})
	})
	Section(c, "FormError: a save that did not go through")
	FormError(c, "Name and content are required.")
	Section(c, "UserBubble: a short message, and one that wraps")
	UserBubble(c, "Summarize the release notes.")
	UserBubble(c, "Summarize the release notes in three bullet points, and keep each one under a line so the bubble wraps across the width of the palette.")
	Section(c, "Pane: the window pane holds its content, with a material or without one")
	ui.Column(c).Height(90).Children(func() {
		Pane(c, func() {
			ui.Column(c).Grow(1).Center().Children(func() {
				ui.Text(c, "Pane body").FontSize(13).TextColor(Tokens(c).Strong)
			})
		})
	})
}
